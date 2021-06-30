package ngtypes

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/c0mm4nd/rlp"
	"math/big"
	"runtime"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/go-randomx"

	"golang.org/x/crypto/sha3"

	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("types")

type Block struct {
	Header *BlockHeader
	Txs    []*Tx
	Subs   []*BlockHeader
}

func NewBlock(network Network, height uint64, timestamp uint64, prevBlockHash, txTrieHash, subTrieHash, difficulty,
	nonce []byte, txs []*Tx, subs []*BlockHeader) *Block {
	return &Block{
		Header: &BlockHeader{
			Network:       network,
			Height:        height,
			Timestamp:     timestamp,
			PrevBlockHash: prevBlockHash,
			TxTrieHash:    txTrieHash,
			SubTrieHash:   subTrieHash,
			Difficulty:    difficulty,
			Nonce:         nonce,
		},
		Txs:  txs,
		Subs: subs,
	}
}

func NewBlockFromHeader(blockHeader *BlockHeader, txs []*Tx, subs []*BlockHeader) *Block {
	return &Block{
		Header: blockHeader,
		Txs:    txs,
		Subs:   subs,
	}
}

// NewBlockFromPoWRaw will apply the raw pow of header and txs to the block.
func NewBlockFromPoWRaw(raw []byte, txs []*Tx, subs []*BlockHeader) (*Block, error) {
	//lenRaw := NetSize +  // 1
	//	HeightSize+        // 8
	//	TimestampSize +    // +
	//	HashSize +         // 32
	//	HashSize +         // +
	//	HashSize +         // +
	//  DiffSize +         // +
	//	NonceSize          // 8
	//                     // = 145
	if len(raw) != 153 {
		return nil, fmt.Errorf("wrong length of PoW raw bytes")
	}

	newBlock := NewBlock(
		Network(raw[0]),
		binary.LittleEndian.Uint64(raw[1:9][:]),
		binary.LittleEndian.Uint64(raw[9:17]),
		raw[17:49],
		raw[49:81],
		raw[81:113],
		bytes.TrimLeft(utils.ReverseBytes(raw[113:145]), string(byte(0))), // remove left padding
		raw[145:153],
		txs,
		subs,
	)

	if err := newBlock.verifyNonce(); err != nil {
		return nil, err
	}

	return newBlock, nil
}

// NewBareBlock will return an unsealing block and
// then you need to add txs and seal with the correct N.
func NewBareBlock(network Network, height uint64, blockTime uint64, prevBlockHash []byte, diff *big.Int) *Block {
	return NewBlock(
		network,
		height,
		blockTime,
		prevBlockHash,
		make([]byte, HashSize),
		make([]byte, HashSize),
		diff.Bytes(),
		make([]byte, NonceSize),
		make([]*Tx, 0),
		[]*BlockHeader{},
	)
}

// IsUnsealing checks whether the block is unsealing.
func (x *Block) IsUnsealing() bool {
	return x.Header.TxTrieHash != nil
}

// IsSealed checks whether the block is sealed.
func (x *Block) IsSealed() bool {
	return x.Header.Nonce != nil
}

// IsHead will check whether the Block is the head(checkpoint).
func (x *Block) IsHead() bool {
	return x.Header.Height%BlockCheckRound == 0
}

// IsTail will check whether the Block is the tail(the one before head).
func (x *Block) IsTail() bool {
	return (x.Header.Height+1)%BlockCheckRound == 0
}

// IsGenesis will check whether the Block is the genesis block.
func (x *Block) IsGenesis() bool {
	return bytes.Equal(x.GetHash(), GetGenesisBlockHash(x.Header.Network))
}

// GetPoWRawHeader will return a complete raw for block hash.
// When nonce is not nil, the RawHeader will use the nonce param not the x.Nonce.
func (x *Block) GetPoWRawHeader(nonce []byte) []byte {
	//lenRaw := NetSize +  // 1
	//	HeightSize+        // 8
	//	TimestampSize +    // +
	//	HashSize +         // 32
	//	HashSize +         // +
	//	HashSize +         // +
	//  DiffSize +         // +
	//	NonceSize          // 8
	//                     // = 145
	raw := make([]byte, 153)

	raw[0] = byte(x.Header.Network)
	binary.LittleEndian.PutUint64(raw[1:], x.Header.Height)
	binary.LittleEndian.PutUint64(raw[9:17], x.Header.Timestamp)
	copy(raw[17:49], x.Header.PrevBlockHash)
	copy(raw[49:81], x.Header.TxTrieHash)
	copy(raw[81:113], x.Header.SubTrieHash)
	copy(raw[113:145], utils.ReverseBytes(x.Header.Difficulty)) // uint256

	if nonce == nil {
		copy(raw[145:153], x.Header.Nonce)
	} else {
		copy(raw[145:153], nonce)
	}

	return raw
}

// PowHash will help you get the pow hash of block.
func (x *Block) PowHash() []byte {
	cache, err := randomx.AllocCache(randomx.FlagJIT)
	if err != nil {
		panic(err)
	}
	defer randomx.ReleaseCache(cache)

	randomx.InitCache(cache, x.Header.PrevBlockHash)
	ds, err := randomx.AllocDataset(randomx.FlagJIT)
	if err != nil {
		panic(err)
	}
	defer randomx.ReleaseDataset(ds)

	count := randomx.DatasetItemCount()
	var wg sync.WaitGroup
	var workerNum = uint32(runtime.NumCPU())
	for i := uint32(0); i < workerNum; i++ {
		wg.Add(1)
		a := (count * i) / workerNum
		b := (count * (i + 1)) / workerNum
		go func() {
			defer wg.Done()
			randomx.InitDataset(ds, cache, a, b-a)
		}()
	}
	wg.Wait()

	vm, err := randomx.CreateVM(cache, ds, randomx.FlagJIT)
	if err != nil {
		panic(err)
	}
	defer randomx.DestroyVM(vm)

	return randomx.CalculateHash(vm, x.GetPoWRawHeader(nil))
}

// ToUnsealing converts a bare block to an unsealing block
func (x *Block) ToUnsealing(txsWithGen []*Tx) error {
	if txsWithGen[0].Type != GenerateTx {
		return fmt.Errorf("first tx shall be a generate")
	}

	for i := 1; i < len(txsWithGen); i++ {
		if txsWithGen[i].Type == GenerateTx {
			return fmt.Errorf("except first, other tx shall not be a generate")
		}
	}

	txTrie := NewTxTrie(txsWithGen)
	x.Header.TxTrieHash = txTrie.TrieRoot()
	x.Txs = txsWithGen

	return nil
}

// ToSealed converts an unsealing block to a sealed block.
func (x *Block) ToSealed(nonce []byte) (*Block, error) {
	if !x.IsUnsealing() {
		return nil, fmt.Errorf("the block is bare")
	}

	if len(nonce) != NonceSize {
		return nil, fmt.Errorf("nonce length %d is incorrect", len(nonce))
	}

	x.Header.Nonce = nonce

	return x, nil
}

// verifyNonce will verify whether the nonce meets the target.
func (x *Block) verifyNonce() error {
	diff := new(big.Int).SetBytes(x.Header.Difficulty)
	target := new(big.Int).Div(MaxTarget, diff)

	if new(big.Int).SetBytes(x.PowHash()).Cmp(target) < 0 {
		return nil
	}

	return fmt.Errorf("block@%d's nonce %x is invalid", x.Header.Height, x.Header.Nonce)
}

// GetActualDiff returns the diff decided by nonce.
func (x *Block) GetActualDiff() *big.Int {
	return new(big.Int).Div(MaxTarget, new(big.Int).SetBytes(x.PowHash()))
}

func (x *Block) GetHeader() *BlockHeader {
	return x.Header
}

func (x *Block) GetTxs() []*Tx {
	return x.Txs
}

// CheckError will check the errors in block inner fields.
func (x *Block) CheckError() error {
	//if x.Network != Network {
	//	return fmt.Errorf("block's network id is incorrect")
	//}
	// DONE: do network check on consensus

	if len(x.Header.PrevBlockHash) != HashSize {
		return fmt.Errorf("block%d's PrevBlockHash length is incorrect", x.Header.Height)
	}

	if len(x.Header.TxTrieHash) != HashSize {
		return fmt.Errorf("block%d's TrieHash length is incorrect", x.Header.Height)
	}

	if len(x.Header.Nonce) != NonceSize {
		return fmt.Errorf("block%d's Nonce length is incorrect", x.Header.Height)
	}

	if x.Header.Timestamp > uint64(time.Now().Unix()) {
		return fmt.Errorf("block%d's timestamp %d is invalid", x.Header.Height, x.Header.Timestamp)
	}

	if !x.IsSealed() {
		return fmt.Errorf("block@%d has not sealed with nonce", x.Header.Height)
	}

	txTrie := NewTxTrie(x.Txs)
	if !bytes.Equal(txTrie.TrieRoot(), x.Header.TxTrieHash) {
		return fmt.Errorf("the merkle tree in block@%d is invalid", x.Header.Height)
	}

	err := x.verifyNonce()
	if err != nil {
		return err
	}

	err = x.verifyNonce()
	if err != nil {
		return err
	}

	return nil
}

// GetHash will help you get the hash of block.
func (x *Block) GetHash() []byte {
	raw, err := rlp.EncodeToBytes(x.Header)
	if err != nil {
		panic(err)
	}

	hash := sha3.Sum256(raw)

	return hash[:]
}

// GetPrevHash is a helper to get the prev block hash from block header.
func (x *Block) GetPrevHash() []byte {
	return x.Header.PrevBlockHash
}

func (x *Block) Equals(other *Block) (bool, error) {
	if eq, _ := x.Header.Equals(other.Header); !eq {
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
