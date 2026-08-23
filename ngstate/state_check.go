package ngstate

import (
	"bytes"
	"math/big"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
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
		panic("shouldnt check generate tx in this func")

	case ngtypes.DestroyTx: // destroy
		if err := checkDestroy(txn, tx); err != nil {
			return err
		}

	case ngtypes.TransactTx: // transact
		if err := checkTransaction(txn, tx); err != nil {
			return err
		}

	case ngtypes.CommitTx: // edit
		if err := checkCommit(txn, tx); err != nil {
			return err
		}

	case ngtypes.ActivateTx: // lock
		if err := checkActivate(txn, tx); err != nil {
			return err
		}

	case ngtypes.DeactivateTx: // unlock
		if err := checkDeactivate(txn, tx); err != nil {
			return err
		}

	default:
		return ngtypes.ErrTxTypeInvalid
	}

	return nil
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

// checkDestroy checks destroy tx: the From address clears its own slot,
// which must exist, be inactive and unreferenced
func checkDestroy(txn *bbolt.Tx, destroyTx *ngtypes.FullTx) error {
	if err := destroyTx.CheckDestroy(keyResolver(txn)); err != nil {
		return err
	}

	from, err := fromWithBalance(txn, destroyTx, destroyTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getContract(txn, from)
	if err != nil {
		return err
	}

	if slot.IsActive() {
		return ErrContractActive
	}
	if refs := getRefCount(slot); refs > 0 {
		return errors.Wrapf(ErrContractRefdBy, "%d dependent contract(s)", refs)
	}

	return nil
}

// checkTransaction checks normal transaction tx
func checkTransaction(txn *bbolt.Tx, transactionTx *ngtypes.FullTx) error {
	if err := transactionTx.CheckTransaction(keyResolver(txn)); err != nil {
		return err
	}

	_, err := fromWithBalance(txn, transactionTx, transactionTx.TotalExpenditure())

	return err
}

// checkCommit checks commit tx: a dry-run of the whole patch
// application; the first commit on an address opens its contract slot
func checkCommit(txn *bbolt.Tx, commitTx *ngtypes.FullTx) error {
	if err := commitTx.CheckCommit(keyResolver(txn)); err != nil {
		return err
	}

	from, err := fromWithBalance(txn, commitTx, commitTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getContract(txn, from)
	if err == nil && slot.IsActive() {
		// an active contract is immutable
		return ErrContractActive
	}

	newSource, err := ngtypes.DecodeCommitCode(commitTx.Extra)
	if err != nil {
		return err
	}
	if len(newSource) > ngtypes.MaxContractSourceSize {
		return errors.Wrapf(ErrSourceTooLarge, "%d bytes exceed the cap %d",
			len(newSource), ngtypes.MaxContractSourceSize)
	}

	return nil
}

// checkActivate checks activate tx
func checkActivate(txn *bbolt.Tx, activateTx *ngtypes.FullTx) error {
	if err := activateTx.CheckActivate(keyResolver(txn)); err != nil {
		return err
	}

	from, err := fromWithBalance(txn, activateTx, activateTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getContract(txn, from)
	if err != nil {
		return err
	}

	if slot.IsActive() {
		return ErrContractActive
	}

	// activating turns the vm on, so the contract text must compile and
	// every declared module dependency must be an active contract
	if len(slot.Source) != 0 {
		if _, err := LoadContractWasm(slot.Source); err != nil {
			return err
		}

		deps, err := extractContractDeps(slot.Source)
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
	}

	return nil
}

// checkDeactivate checks unactivate tx
func checkDeactivate(txn *bbolt.Tx, deactivateTx *ngtypes.FullTx) error {
	if err := deactivateTx.CheckDeactivate(keyResolver(txn)); err != nil {
		return err
	}

	from, err := fromWithBalance(txn, deactivateTx, deactivateTx.TotalExpenditure())
	if err != nil {
		return err
	}

	slot, err := getContract(txn, from)
	if err != nil {
		return err
	}

	if !slot.IsActive() {
		return ErrContractNotActive
	}

	// a depended-on module cannot deactivate
	if refs := getRefCount(slot); refs > 0 {
		return errors.Wrapf(ErrContractRefdBy, "%d dependent contract(s)", refs)
	}

	return nil
}
