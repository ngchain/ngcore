package ngtypes

import (
	"bytes"
	"math/big"
	"sort"
	"time"

	"github.com/ngchain/astrobwt"

	"github.com/c0mm4nd/rlp"
	"github.com/ngchain/ngcore/utils"
	logging "github.com/ngchain/zap-log"
	"github.com/pkg/errors"
)

var log = logging.Logger("types")

var (
	ErrBlockNoGen      = errors.New("the first tx in one block is required to be a generate tx")
	ErrBlockOnlyOneGen = errors.New("tx should have only one tx")

	ErrBlockNoHeader          = errors.New("block header is nil")
	ErrBlockDiffInvalid       = errors.New("invalid block diff")
	ErrBlockPrevHashInvalid   = errors.New("invalid block prev hash")
	ErrBlockTxTrieHashInvalid = errors.New("invalid block tx trie hash")
	// ErrBlockWitnessRootInvalid rejects a header whose witness
	// commitment does not match the carried signature envelopes
	ErrBlockWitnessRootInvalid = errors.New("invalid block witness root")
	// ErrBlockTxsExcess rejects a block carrying more txs than consensus allows
	ErrBlockTxsExcess = errors.New("block carries too many txs")
	// ErrBlockBytesExcess rejects a block bigger than consensus allows
	ErrBlockBytesExcess      = errors.New("block is too large")
	ErrBlockTimestampInvalid = errors.New("invalid block timestamp")

	ErrBlockNotSealed = errors.New("the block is not sealed")

	// ErrBlockDuplicateTx rejects a block that carries the same txid twice.
	// Because a tx's id excludes its Salt, two reveals of the same content
	// under different salts share an id; without this guard they would both
	// apply — a self-funded double-execution. ErrBlockDuplicateCommit is the
	// same guard for commitments (two identical hashes would double-charge).
	ErrBlockDuplicateTx     = errors.New("block carries a duplicate txid")
	ErrBlockDuplicateCommit = errors.New("block carries a duplicate commitment")
	// ErrBlockCommitsExcess rejects a block with more commitments than allowed
	ErrBlockCommitsExcess = errors.New("block carries too many commitments")

	// ErrBlockUnclesInvalid rejects a block whose uncle set breaks a
	// context-free rule (bad commitment, over the cap, duplicated, or an
	// uncle header that fails its own standalone pow/format check)
	ErrBlockUnclesInvalid = errors.New("invalid block uncles")

	// ErrBlockStateRootInvalid rejects a header whose committed post-state
	// root is malformed (wrong length) or, at the state layer, does not
	// match the root produced by applying the block
	ErrBlockStateRootInvalid = errors.New("invalid block state root")
)

// FullBlock is an implement of Block the base unit of the blockchain and the container of the txs, which
// provides the safety assurance by the hashes in the header
type FullBlock struct {
	*BlockHeader
	Txs    []*FullTx
	Uncles []*BlockHeader
	// Commits are the blind commitments of the mandatory commit-reveal
	// private mempool packed at this height. They are folded into the
	// existing TxTrieHash (via ContentRoot), so the header/pow preimage is
	// unchanged yet binds them too.
	Commits []*Commitment `rlp:"optional"`
}

// NewBlock creates a new Block
func NewBlock(network Network, height uint64, timestamp uint64, prevBlockHash, txTrieHash, witnessRoot, difficulty,
	nonce []byte, txs []*FullTx) *FullBlock {
	return &FullBlock{
		BlockHeader: &BlockHeader{
			Network:       network,
			Height:        height,
			Timestamp:     timestamp,
			PrevBlockHash: prevBlockHash,
			TxTrieHash:    txTrieHash,
			WitnessRoot:   witnessRoot,
			Difficulty:    difficulty,
			Coinbase:      make([]byte, AddressSize), // set by SetCoinbase before sealing
			UnclesHash:    make([]byte, HashSize),    // no uncles until SetUncles
			StateRoot:     make([]byte, HashSize),    // set to the post-state root before sealing
			Nonce:         nonce,
		},
		Txs: txs,
	}
}

// SetCoinbase records the miner's address in the header. It must run
// BEFORE sealing (Coinbase is part of the pow preimage) and must match the
// recipient of the block's own generate tx.
func (x *FullBlock) SetCoinbase(addr Address) {
	x.BlockHeader.Coinbase = addr[:]
}

// CalcUnclesHash commits to a block's uncle headers: all-zero when there
// are none, otherwise the blake3 over the concatenated uncle hashes (in
// the given order, which SetUncles fixes deterministically).
func CalcUnclesHash(uncles []*BlockHeader) []byte {
	if len(uncles) == 0 {
		return make([]byte, HashSize)
	}
	buf := make([]byte, 0, len(uncles)*HashSize)
	for _, u := range uncles {
		buf = append(buf, u.GetHash()...)
	}
	return utils.Hash256(buf)
}

// SetUncles attaches uncle headers to a bare/unsealing block and refreshes
// the header commitment. It must run BEFORE sealing, since UnclesHash is
// part of the pow preimage. Uncles are sorted by hash for a canonical,
// grind-resistant order.
func (x *FullBlock) SetUncles(uncles []*BlockHeader) {
	sort.Slice(uncles, func(i, j int) bool {
		return bytes.Compare(uncles[i].GetHash(), uncles[j].GetHash()) < 0
	})
	x.Uncles = uncles
	x.BlockHeader.UnclesHash = CalcUnclesHash(uncles)
}

// CalcWitnessRoot commits the signature envelopes of a block's txs AND its
// commitments, in order: blake3 over the per-item blake3 of the witness bytes.
// The header carries it so witness data is immutable in the live chain, yet
// REPLACEABLE for settled history (pruning, aggregate proofs) without touching
// any txid. Covering the commitment sigs too closes the same malleability the
// tx witness commitment closes — the content root excludes Sign, so without
// this a committer could swap a commit's sig bytes under one header.
func CalcWitnessRoot(txs []*FullTx, commits []*Commitment) []byte {
	buf := make([]byte, 0, (len(txs)+len(commits))*HashSize)
	for _, tx := range txs {
		buf = append(buf, utils.Hash256(tx.Sign)...)
	}
	for _, commit := range commits {
		buf = append(buf, utils.Hash256(commit.Sign)...)
	}

	return utils.Hash256(buf)
}

// NewBareBlock will return an unsealing block and
// then you need to add txs and seal with the correct N.
func NewBareBlock(network Network, height uint64, blockTime uint64, prevBlockHash []byte, diff *big.Int) *FullBlock {
	return NewBlock(
		network,
		height,
		blockTime,
		prevBlockHash,
		make([]byte, HashSize),
		make([]byte, HashSize),
		diff.Bytes(),
		make([]byte, NonceSize),
		make([]*FullTx, 0),
	)
}

// IsUnsealing checks whether the block is unsealing.
func (x *FullBlock) IsUnsealing() bool {
	return x.BlockHeader.TxTrieHash != nil
}

// IsSealed checks whether the block is sealed.
func (x *FullBlock) IsSealed() bool {
	return x.BlockHeader.Nonce != nil
}

// IsHead will check whether the Block is the head(checkpoint).
func (x *FullBlock) IsHead() bool {
	return x.BlockHeader.Height%BlockCheckRound == 0
}

// IsTail will check whether the Block is the tail(the one before head).
func (x *FullBlock) IsTail() bool {
	return (x.BlockHeader.Height+1)%BlockCheckRound == 0
}

// IsGenesis will check whether the Block is the genesis block.
func (x *FullBlock) IsGenesis() bool {
	return bytes.Equal(x.GetHash(), GetGenesisBlock(x.BlockHeader.Network).GetHash())
}

// GetPoWRawHeader will return a complete raw for block hash.
// When nonce is not nil, the RawHeader will use the nonce param not the x.Nonce.
func (x *FullBlock) GetPoWRawHeader(nonce []byte) []byte {
	return x.BlockHeader.GetPoWRawHeader(nonce)
}

// PowHash will help you get the pow hash of block.
func (x *FullBlock) PowHash() []byte {
	hash := astrobwt.POW_0alloc(x.GetPoWRawHeader(nil))
	return hash[:]
}

// ToUnsealing converts a bare block to an unsealing block. The first tx is
// the miner's (signed) generate; a block may additionally carry UNSIGNED
// generate txs — the uncle rewards, one per referenced uncle — which are
// validated as a set against the uncles by the state layer. Any further
// SIGNED generate is rejected (only the miner signs one).
func (x *FullBlock) ToUnsealing(txsWithGen []*FullTx) error {
	if txsWithGen[0].Type != GenerateTx {
		return ErrBlockNoGen
	}

	for i := 1; i < len(txsWithGen); i++ {
		if txsWithGen[i].Type == GenerateTx {
			// only the miner's own generate is signed; uncle-reward
			// generates are system mints and must be unsigned
			if txsWithGen[i].IsSigned() {
				return ErrBlockOnlyOneGen
			}
			continue
		}

		if txsWithGen[i].Height != x.Height {
			return ErrTxExtraInvalid
		}
	}

	// canonicalize the tx order FIRST, then commit over it: the witness root
	// must match CheckError's recomputation, which runs over the stored
	// (sorted) x.Txs. NewTxTrie sorts in place, so txsWithGen and x.Txs share
	// the one canonical order
	x.Txs = NewTxTrie(txsWithGen)
	// the content root covers BOTH the txs and this block's commitments,
	// deterministically ordered; it lands in the existing TxTrieHash so the
	// pow preimage binds commitments without any header change
	x.BlockHeader.TxTrieHash = ContentRoot(x.Txs, x.Commits)
	x.BlockHeader.WitnessRoot = CalcWitnessRoot(x.Txs, x.Commits)

	return nil
}

// SetCommits attaches the block's commitments before sealing. They fold into
// the content root computed by ToUnsealing, so this must run first.
func (x *FullBlock) SetCommits(commits []*Commitment) {
	x.Commits = commits
}

var (
	ErrBlockSealBare = errors.New("sealing a bare block")
	ErrInvalidNonce  = errors.New("nonce is invalid")
)

// ToSealed converts an unsealing block to a sealed block.
func (x *FullBlock) ToSealed(nonce []byte) error {
	if !x.IsUnsealing() {
		return ErrBlockSealBare
	}

	if len(nonce) != NonceSize {
		return errors.Wrapf(ErrInvalidNonce, "nonce length %d is incorrect", len(nonce))
	}

	x.BlockHeader.Nonce = nonce

	return nil
}

// verifyNonce will verify whether the nonce meets the target.
func (x *FullBlock) verifyNonce() error {
	if x.Height == 0 {
		// ignore genesis block nonce check
		return nil
	}

	diff := new(big.Int).SetBytes(x.BlockHeader.Difficulty)
	target := new(big.Int).Div(MaxTarget, diff)

	if new(big.Int).SetBytes(x.PowHash()).Cmp(target) < 0 {
		return nil
	}

	return errors.Wrapf(ErrInvalidNonce, "block@%d's nonce %x is invalid", x.BlockHeader.Height, x.BlockHeader.Nonce)
}

// GetActualDiff returns the diff decided by nonce.
func (x *FullBlock) GetActualDiff() *big.Int {
	return new(big.Int).Div(MaxTarget, new(big.Int).SetBytes(x.PowHash()))
}

// CheckError will check the errors in block inner fields.
func (x *FullBlock) CheckError() error {
	// capacity: consensus bounds, checked before anything expensive
	if len(x.Txs) > MaxBlockTxCount {
		return errors.Wrapf(ErrBlockTxsExcess, "%d txs exceed the cap %d", len(x.Txs), MaxBlockTxCount)
	}
	if len(x.Commits) > MaxBlockCommitCount {
		return errors.Wrapf(ErrBlockCommitsExcess, "%d commitments exceed the cap %d", len(x.Commits), MaxBlockCommitCount)
	}
	if raw, err := rlp.EncodeToBytes(x); err != nil {
		return err
	} else if len(raw) > MaxBlockBytes {
		return errors.Wrapf(ErrBlockBytesExcess, "%d bytes exceed the cap %d", len(raw), MaxBlockBytes)
	}

	// if x.Network != Network {
	//	return fmt.Errorf("block's network id is incorrect")
	// }
	// DONE: do network check on consensus

	if len(x.BlockHeader.PrevBlockHash) != HashSize {
		return errors.Wrapf(ErrBlockPrevHashInvalid, "block%d's PrevBlockHash length is incorrect", x.BlockHeader.Height)
	}

	if len(x.BlockHeader.TxTrieHash) != HashSize {
		return errors.Wrapf(ErrBlockTxTrieHashInvalid, "block%d's TrieHash length is incorrect", x.BlockHeader.Height)
	}

	if len(x.BlockHeader.Nonce) != NonceSize {
		return errors.Wrapf(ErrInvalidNonce, "block%d's Nonce length is incorrect", x.BlockHeader.Height)
	}

	// allow a small clock drift; a block rejected as futuristic becomes
	// acceptable once the local clock catches up
	if x.BlockHeader.Timestamp > uint64(time.Now().UnixMilli())+TimestampDriftTolerance {
		return errors.Wrapf(ErrBlockTimestampInvalid, "block%d's timestamp %d is too far in the future", x.BlockHeader.Height, x.BlockHeader.Timestamp)
	}

	if !x.IsSealed() {
		return errors.Wrapf(ErrBlockNotSealed, "block@%d has not sealed with nonce", x.BlockHeader.Height)
	}

	// every commitment must be well-formed and self-consistent before it can
	// count toward the content root (32B Hash, non-nil Fee, matching Height,
	// verifying signature). The stateful checks (fee charge, reveal window)
	// live in the state layer
	for _, commit := range x.Commits {
		if err := commit.CheckError(x.BlockHeader.Height, nil); err != nil {
			return errors.Wrapf(err, "block@%d carries an invalid commitment", x.BlockHeader.Height)
		}
	}

	// reject duplicates: two txs with the same id (a salt-varying double
	// reveal) or two identical commitments would each be applied/charged twice
	seenTx := make(map[string]struct{}, len(x.Txs))
	for _, tx := range x.Txs {
		if tx.Type == GenerateTx {
			continue // generates are validated as a set, not by unique id
		}
		id := string(tx.GetHash())
		if _, dup := seenTx[id]; dup {
			return errors.Wrapf(ErrBlockDuplicateTx, "block@%d carries txid %x twice", x.BlockHeader.Height, tx.GetHash())
		}
		seenTx[id] = struct{}{}
	}
	seenCommit := make(map[string]struct{}, len(x.Commits))
	for _, commit := range x.Commits {
		id := string(commit.Hash)
		if _, dup := seenCommit[id]; dup {
			return errors.Wrapf(ErrBlockDuplicateCommit, "block@%d carries commitment %x twice", x.BlockHeader.Height, commit.Hash)
		}
		seenCommit[id] = struct{}{}
	}

	// the content root covers the txs AND the commitments, in one canonical
	// hash-sorted order
	if root := ContentRoot(x.Txs, x.Commits); !bytes.Equal(root, x.BlockHeader.TxTrieHash) {
		return errors.Wrapf(
			ErrBlockTxTrieHashInvalid,
			"the content merkle tree in block@%d is invalid: %x != %x",
			x.BlockHeader.Height,
			root,
			x.BlockHeader.TxTrieHash,
		)
	}

	// the witness commitment pins the signature envelopes: without it,
	// the same txids with different signature bytes would yield two
	// different valid blocks under one header
	if !bytes.Equal(CalcWitnessRoot(x.Txs, x.Commits), x.BlockHeader.WitnessRoot) {
		return errors.Wrapf(ErrBlockWitnessRootInvalid,
			"block@%d's witness root does not match its txs", x.BlockHeader.Height)
	}

	if len(x.BlockHeader.Coinbase) != AddressSize {
		return errors.Wrapf(ErrBlockUnclesInvalid, "block@%d's Coinbase length is incorrect", x.BlockHeader.Height)
	}

	// uncle commitment must match the carried uncle headers, and the set
	// must be bounded, self-consistent and duplicate-free. Parentage,
	// difficulty-for-slot and dedup-vs-chain are enforced in the chain layer.
	if len(x.BlockHeader.UnclesHash) != HashSize {
		return errors.Wrapf(ErrBlockUnclesInvalid, "block@%d's UnclesHash length is incorrect", x.BlockHeader.Height)
	}

	// the post-state commitment must be a full 32-byte root on every block,
	// genesis included; the match against the applied state is a stateful
	// check the chain layer runs after State.Upgrade
	if len(x.BlockHeader.StateRoot) != HashSize {
		return errors.Wrapf(ErrBlockStateRootInvalid, "block@%d's StateRoot length is incorrect", x.BlockHeader.Height)
	}

	if len(x.Uncles) > MaxUncles {
		return errors.Wrapf(ErrBlockUnclesInvalid, "block@%d carries %d uncles over the cap %d", x.BlockHeader.Height, len(x.Uncles), MaxUncles)
	}
	if !bytes.Equal(CalcUnclesHash(x.Uncles), x.BlockHeader.UnclesHash) {
		return errors.Wrapf(ErrBlockUnclesInvalid, "block@%d's uncle commitment does not match its uncles", x.BlockHeader.Height)
	}
	seenUncle := make(map[string]struct{}, len(x.Uncles))
	for _, u := range x.Uncles {
		if err := u.checkStandaloneError(); err != nil {
			return errors.Wrapf(ErrBlockUnclesInvalid, "block@%d has an invalid uncle: %s", x.BlockHeader.Height, err)
		}
		uh := string(u.GetHash())
		if _, dup := seenUncle[uh]; dup {
			return errors.Wrapf(ErrBlockUnclesInvalid, "block@%d references uncle %x twice", x.BlockHeader.Height, u.GetHash())
		}
		seenUncle[uh] = struct{}{}
		if bytes.Equal(u.GetHash(), x.BlockHeader.PrevBlockHash) {
			return errors.Wrapf(ErrBlockUnclesInvalid, "block@%d references its own parent as an uncle", x.BlockHeader.Height)
		}
	}

	err := x.verifyNonce()
	if err != nil {
		return err
	}

	return nil
}

// GetHash will help you get the hash of block.
func (x *FullBlock) GetHash() []byte {
	raw, err := rlp.EncodeToBytes(x.BlockHeader)
	if err != nil {
		panic(err)
	}

	return utils.Hash256(raw)
}

func (x *FullBlock) Equals(other *FullBlock) (bool, error) {
	if eq, _ := x.BlockHeader.Equals(other.BlockHeader); !eq {
		return false, nil
	}
	if len(x.Txs) != len(other.Txs) {
		return false, nil
	}

	for i := 0; i < len(x.Txs); i++ {
		if eq, err := x.Txs[i].Equals(other.Txs[i]); !eq {
			return false, err
		}
	}

	return true, nil
}

func (x *FullBlock) GetTx(i int) Tx {
	if i >= len(x.Txs) || i < 0 {
		return nil
	}

	return x.Txs[i]
}
