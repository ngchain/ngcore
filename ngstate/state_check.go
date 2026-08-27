package ngstate

import (
	"bytes"
	"math/big"

	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var ErrTxrBalanceInsufficient = errors.New("address balance is not sufficient for the tx")

// CheckBlockTxs will check all requirements for txs in block. This is the
// universal validation gate: every path that applies state (fast import,
// reorg unwind, full rebuild) runs it before crediting any balance.
func CheckBlockTxs(txn *bbolt.Tx, block *ngtypes.FullBlock) error {
	// generates are validated as a SET (their order is not fixed): the one
	// signed miner generate plus the unsigned uncle-reward generates
	if err := checkBlockGenerates(txn, block); err != nil {
		return err
	}

	for i := 0; i < len(block.Txs); i++ {
		tx := block.Txs[i]
		if tx.Type == ngtypes.GenerateTx {
			continue // handled by checkBlockGenerates
		}

		if !tx.IsSigned() {
			return ngtypes.ErrTxUnsigned
		}
		if len(tx.Extra) > ngtypes.TxMaxExtraSize {
			return ngtypes.ErrTxExtraExcess
		}
		if err := CheckTx(txn, tx); err != nil {
			return err
		}
		// burn-only base fee (ForkFeeMarket): once the fork is active, every
		// non-generate tx must pay Fee >= block.BaseFee * len(rlp(tx)). The fee
		// is still fully burned by chargeFrom — this only raises the minimum.
		// Pre-fork this is not enforced (unchanged behavior). Gated on the
		// block's height + network so non-block CheckTx callers are unaffected.
		if err := checkTxBaseFee(block, tx); err != nil {
			return err
		}
	}

	return nil
}

// checkTxBaseFee enforces the post-fork per-tx burn-only base-fee minimum:
// Fee >= block.BaseFee * len(rlp(tx)). It is a no-op before ForkFeeMarket is
// active at the block's height. Only reachable for non-generate txs.
func checkTxBaseFee(block *ngtypes.FullBlock, tx *ngtypes.FullTx) error {
	if !ngtypes.IsForkActive(block.BlockHeader.Network, ngtypes.ForkFeeMarket, block.GetHeight()) {
		return nil
	}

	raw, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}
	baseFee := new(big.Int).SetBytes(block.BlockHeader.BaseFee)
	minFee := new(big.Int).Mul(baseFee, big.NewInt(int64(len(raw))))
	if tx.Fee.Cmp(minFee) < 0 {
		return errors.Wrapf(ngtypes.ErrTxFeeBelowBaseFee,
			"tx fee %s < base-fee minimum %s (baseFee %s * %d bytes) at height %d",
			tx.Fee, minFee, baseFee, len(raw), block.GetHeight())
	}

	return nil
}

// checkBlockGenerates validates the block's generate txs as a set. Exactly
// one SIGNED miner generate must pay header.Coinbase the block reward
// (standard generate rules), and there must be exactly one UNSIGNED
// uncle-reward generate per referenced uncle, paying that uncle's Coinbase
// the depth-decayed UncleReward. They are matched as a multiset on
// (recipient, amount) so two orphans from the same miner still pair up. No
// other generate is allowed. This is what lets handleGenerate blindly mint.
func checkBlockGenerates(txn *bbolt.Tx, block *ngtypes.FullBlock) error {
	height := block.GetHeight()

	var primary *ngtypes.FullTx
	uncleGens := make([]*ngtypes.FullTx, 0, len(block.Uncles))
	for _, tx := range block.Txs {
		if tx.Type != ngtypes.GenerateTx {
			continue
		}
		if tx.IsSigned() {
			if primary != nil {
				return errors.Wrap(ngtypes.ErrRewardInvalid, "more than one signed miner generate")
			}
			primary = tx
		} else {
			uncleGens = append(uncleGens, tx)
		}
	}

	if primary == nil {
		return errors.Wrap(ngtypes.ErrRewardInvalid, "block has no signed miner generate")
	}
	if !bytes.Equal(primary.To[:], block.BlockHeader.Coinbase) {
		return errors.Wrap(ngtypes.ErrRewardInvalid, "miner generate does not pay the header coinbase")
	}
	if err := primary.CheckGenerate(height, keyResolver(txn)); err != nil {
		return err
	}

	if len(uncleGens) != len(block.Uncles) {
		return errors.Wrapf(ngtypes.ErrRewardInvalid,
			"%d uncle-reward generates for %d uncles", len(uncleGens), len(block.Uncles))
	}
	// expected (recipient||amount) multiset from the declared uncles; the
	// 32-byte address prefix makes the concatenation unambiguous
	want := make(map[string]int, len(block.Uncles))
	for _, u := range block.Uncles {
		want[string(u.Coinbase)+ngtypes.UncleReward(u.Height, height).String()]++
	}
	for _, g := range uncleGens {
		if g.Fee.Sign() != 0 {
			return errors.Wrap(ngtypes.ErrTxFeeInvalid, "uncle-reward generate fee must be zero")
		}
		if g.Height != height {
			return errors.Wrap(ngtypes.ErrRewardInvalid, "uncle-reward generate height mismatch")
		}
		key := string(g.To[:]) + g.Value.String()
		if want[key] == 0 {
			return errors.Wrap(ngtypes.ErrRewardInvalid, "uncle-reward generate matches no uncle")
		}
		want[key]--
	}

	return nil
}

// CheckRevealExceptCommitment runs every admission check an effect tx (reveal)
// must pass EXCEPT the commit-reveal gate: the signature (resolving compact
// envelopes through the on-chain registry), the extra-size bound, the
// type-specific content rules, and that the sender can AFFORD the expenditure.
// The pool uses it at relay-enqueue time — when the reveal's commitment is not
// yet on chain, so full CheckTx would fail on checkReveal — to reject a forged
// or unfunded reveal before it can squat a relay-queue slot. The affordability
// check matters because a recover-envelope signature always "verifies" (it
// recovers whatever key signed it), so only the unfunded derived From exposes a
// forged reveal. No state is mutated.
func CheckRevealExceptCommitment(txn *bbolt.Tx, tx *ngtypes.FullTx) error {
	if !tx.IsSigned() {
		return ngtypes.ErrTxSignInvalid
	}
	if len(tx.Extra) > ngtypes.TxMaxExtraSize {
		return ngtypes.ErrTxExtraExcess
	}

	switch tx.Type {
	case ngtypes.TransactTx:
		return checkTransaction(txn, tx)
	case ngtypes.DeployTx:
		return checkDeploy(txn, tx)
	default:
		return ngtypes.ErrTxTypeInvalid
	}
}

// CheckTx will check the requirements for one tx (except generate tx)
func CheckTx(txn *bbolt.Tx, tx *ngtypes.FullTx) error {
	// check tx is signed
	if !tx.IsSigned() {
		return ngtypes.ErrTxSignInvalid
	}

	// check the tx's extra size is necessary
	if len(tx.Extra) > ngtypes.TxMaxExtraSize {
		return ngtypes.ErrTxExtraExcess
	}

	switch tx.Type {
	case ngtypes.GenerateTx: // generate
		// generates are validated as a block-level set (checkBlockGenerates);
		// a stray generate here — e.g. one gossiped into the tx pool — is
		// rejected, never panicked, so a peer cannot crash the node
		return ngtypes.ErrTxTypeInvalid

	case ngtypes.TransactTx: // transact
		if err := checkReveal(txn, tx); err != nil {
			return err
		}
		if err := checkTransaction(txn, tx); err != nil {
			return err
		}

	case ngtypes.DeployTx: // deploy / upgrade
		if err := checkReveal(txn, tx); err != nil {
			return err
		}
		if err := checkDeploy(txn, tx); err != nil {
			return err
		}

	default:
		return ngtypes.ErrTxTypeInvalid
	}

	return nil
}

// checkReveal enforces the mandatory commit-reveal rule: an effect tx
// (Transact/Deploy/Destroy) is valid ONLY if it carries a non-empty Salt AND
// there is an UNREVEALED commitment on chain whose value-From matches the
// tx's From and whose Hash == blake3(tx.UnheightedHash() ‖ tx.Salt),
// recorded at some height h with revealHeight-CommitWindow <= h <
// revealHeight (STRICTLY earlier — the anti-same-block-reaction rule). The
// reveal height is the tx's own height-lock. GenerateTx is exempt (never
// reaches here). CheckTx runs against current committed state, so a reveal is
// only pool-admissible after its commitment is already on chain.
func checkReveal(txn *bbolt.Tx, tx *ngtypes.FullTx) error {
	if tx.Height == 0 {
		return nil // genesis txs bypass every tx check
	}
	if len(tx.Salt) == 0 {
		return errors.Wrap(ErrTxNotCommitted, "effect tx carries no salt")
	}
	if len(tx.Salt) < ngtypes.MinSaltSize {
		return errors.Wrapf(ErrSaltTooShort, "salt is %d bytes, need >= %d", len(tx.Salt), ngtypes.MinSaltSize)
	}

	from, err := tx.From()
	if err != nil {
		return err
	}

	hash := revealHash(tx)
	if _, ok := findCommit(txn, from, hash, tx.Height); !ok {
		return errors.Wrapf(ErrTxNotCommitted,
			"no in-window unrevealed commitment for %s revealing at height %d", from, tx.Height)
	}

	return nil
}

// revealHash is the commitment preimage of a reveal tx: blake3 over the tx's
// height-independent content hash concatenated with its Salt. Excluding the
// target height lets the same commitment be revealed at any height in the
// window, so a censoring miner cannot pin a reveal to one block.
func revealHash(tx *ngtypes.FullTx) []byte {
	buf := make([]byte, 0, ngtypes.HashSize+len(tx.Salt))
	buf = append(buf, tx.UnheightedHash()...)
	buf = append(buf, tx.Salt...)
	return utils.Hash256(buf)
}

// checkGenerate checks the generate tx
func checkGenerate(txn *bbolt.Tx, generateTx *ngtypes.FullTx, blockHeight uint64) error {
	return generateTx.CheckGenerate(blockHeight, keyResolver(txn))
}

// fromWithBalance derives the From address and checks it can afford the
// expense
func fromWithBalance(txn *bbolt.Tx, tx *ngtypes.FullTx, expense *big.Int) (ngtypes.Address, error) {
	from, err := tx.From()
	if err != nil {
		return ngtypes.Address{}, err
	}

	if getBalance(txn, from).Cmp(expense) < 0 {
		return ngtypes.Address{}, ErrTxrBalanceInsufficient
	}

	return from, nil
}

// checkTransaction checks normal transaction tx
func checkTransaction(txn *bbolt.Tx, transactionTx *ngtypes.FullTx) error {
	if err := transactionTx.CheckTransaction(keyResolver(txn)); err != nil {
		return err
	}

	_, err := fromWithBalance(txn, transactionTx, transactionTx.TotalExpenditure())

	return err
}

// checkDeploy checks a deploy tx (a dry-run of the whole deploy/upgrade):
// the module carried in Extra must decode, fit the size cap, compile, and
// every declared dependency must be a live contract. An EMPTY slot deploys
// the module and goes live at once; a LIVE slot is upgraded UUPS-style —
// the CURRENT code must export the `upgrade` hook or the tx is rejected as
// targeting an immutable contract
func checkDeploy(txn *bbolt.Tx, deployTx *ngtypes.FullTx) error {
	if err := deployTx.CheckDeploy(keyResolver(txn)); err != nil {
		return err
	}

	from, err := fromWithBalance(txn, deployTx, deployTx.TotalExpenditure())
	if err != nil {
		return err
	}

	newSource, err := ngtypes.DecodeCommitCode(deployTx.Extra)
	if err != nil {
		return err
	}
	if len(newSource) > ngtypes.MaxContractSourceSize {
		return errors.Wrapf(ErrSourceTooLarge, "%d bytes exceed the cap %d",
			len(newSource), ngtypes.MaxContractSourceSize)
	}

	slot, err := getContract(txn, from)
	isLive := err == nil && slot.IsActive() && len(slot.Source) != 0

	// EMPTY code -> DESTROY: only a live slot can be destroyed, its current
	// code must opt in via `upgrade` (else immutable & indestructible), and it
	// must be unreferenced
	if len(newSource) == 0 {
		if !isLive {
			return ErrNothingToDestroy
		}
		if !contractHasExport(slot.Source, VMEntryOnUpgrade) {
			return ErrImmutable
		}
		if refs := getRefCount(slot); refs > 0 {
			return errors.Wrapf(ErrContractRefdBy, "%d dependent contract(s)", refs)
		}
		return nil
	}

	// the new module must compile and every declared dependency must be a
	// live contract (checked at deploy AND at upgrade time)
	if _, err := LoadContractWasm(newSource); err != nil {
		return err
	}
	deps, err := extractContractDeps(newSource)
	if err != nil {
		return err
	}
	for _, depAddr := range deps {
		if depAddr.Equals(from) {
			return ErrDepSelf
		}
		depAcc, err := getContract(txn, depAddr)
		if err != nil {
			return errors.Wrapf(err, "unknown dependency contract %s", depAddr)
		}
		if !depAcc.IsActive() || len(depAcc.Source) == 0 {
			return errors.Wrapf(ErrDepNotActive, "contract %s", depAddr)
		}
	}

	// a live slot can only be upgraded if its CURRENT code opts in via an
	// `upgrade` export; otherwise the contract is immutable
	if isLive && !contractHasExport(slot.Source, VMEntryOnUpgrade) {
		return ErrImmutable
	}

	return nil
}
