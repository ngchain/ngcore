package ngtypes

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ngchain/go-randomx"

	"google.golang.org/protobuf/proto"

	"golang.org/x/crypto/sha3"

	"github.com/ngchain/ngcore/ngtypes/ngproto"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("types")

type Block struct {
	Header *ngproto.BlockHeader // do not inherit to avoid Marshal(ngtypes.Block)
	Txs    []*Tx
	Hash   []byte
}

func NewBlock(network ngproto.NetworkType, height uint64, timestamp int64, prevBlockHash, trieHash, difficulty,
	nonce []byte, subs []*ngproto.BlockHeader, txs []*Tx, hash []byte) *Block {
	return &Block{
		Header: &ngproto.BlockHeader{
			Network:       network,
			Height:        height,
			Timestamp:     timestamp,
			PrevBlockHash: prevBlockHash,
			TrieHash:      trieHash,
			Difficulty:    difficulty,
			Nonce:         nonce,
			Subs:          subs,
		},
		Txs:  txs,
		Hash: hash,
	}
}

func NewBlockFromHeader(protoBlockHeader *ngproto.BlockHeader, txs []*Tx) *Block {
	return &Block{
		Header: protoBlockHeader,
		Txs:    txs,
	}
}

func NewBlockFromProto(protoBlock *ngproto.Block) *Block {
	txs := make([]*Tx, len(protoBlock.Txs))
	for i := 0; i < len(protoBlock.Txs); i++ {
		txs[i] = NewTxFromProto(protoBlock.Txs[i])
	}

	return &Block{
		Header: protoBlock.Header,
		Txs:    txs,
	}
}

// NewBlockFromPoWRawWithTxs will apply the raw pow of header and txs to the block.
func NewBlockFromPoWRawWithTxs(raw []byte, txs []*Tx) (*Block, error) {
	//lenRaw := 1 + // network size
	//	HeightSize+
	//	TimestampSize +
	//	HashSize +
	//	HashSize + // unknown length
	//	HashSize +
	//	NonceSize
	if len(raw) != 121 {
		return nil, fmt.Errorf("wrong length of PoW raw bytes")
	}

	newBlock := NewBlock(
		ngproto.NetworkType(raw[0]),
		binary.LittleEndian.Uint64(raw[1:9][:]),
		int64(binary.LittleEndian.Uint64(raw[9:17])),
		raw[17:49],
		raw[49:81],
		bytes.TrimLeft(utils.ReverseBytes(raw[81:113]), string(byte(0))), // remove left padding
		raw[113:121],
		[]*ngproto.BlockHeader{},
		txs,
		nil,
	)
	newBlock.GetHash()

	if err := newBlock.verifyNonce(); err != nil {
		return nil, err
	}

	return newBlock, nil
}

// NewBareBlock will return an unsealing block and
// then you need to add txs and seal with the correct N.
func NewBareBlock(network ngproto.NetworkType, height uint64, blockTime int64, prevBlockHash []byte, diff *big.Int) *Block {
	return NewBlock(
		network,
		height,
		blockTime,
		prevBlockHash,
		make([]byte, HashSize),

		diff.Bytes(),
		make([]byte, NonceSize),
		[]*ngproto.BlockHeader{},
		make([]*Tx, 0),
		nil,
	)
}

func (x *Block) GetProto() *ngproto.Block {
	txs := make([]*ngproto.Tx, len(x.Txs))
	for i := 0; i < len(x.Txs); i++ {
		txs[i] = x.Txs[i].GetProto()
	}

	return &ngproto.Block{
		Header: x.Header,
		Txs:    txs,
	}
}

func (x *Block) Marshal() ([]byte, error) {
	protoBlock := proto.Clone(x.GetProto()).(*ngproto.Block)

	return proto.Marshal(protoBlock)
}

// IsUnsealing checks whether the block is unsealing.
func (x *Block) IsUnsealing() bool {
	return x.Header.GetTrieHash() != nil
}

// IsSealed checks whether the block is sealed.
func (x *Block) IsSealed() bool {
	return x.Header.GetNonce() != nil
}

// IsHead will check whether the Block is the head(checkpoint).
func (x *Block) IsHead() bool {
	return x.Header.GetHeight()%BlockCheckRound == 0
}

// IsTail will check whether the Block is the tail(the one before head).
func (x *Block) IsTail() bool {
	return (x.Header.GetHeight()+1)%BlockCheckRound == 0
}

// IsGenesis will check whether the Block is the genesis block.
func (x *Block) IsGenesis() bool {
	return bytes.Equal(x.GetHash(), GetGenesisBlockHash(x.Header.GetNetwork()))
}

// GetPoWRawHeader will return a complete raw for block hash.
// When nonce is not nil, the RawHeader will use the nonce param not the x.Nonce.
func (x *Block) GetPoWRawHeader(nonce []byte) []byte {
	//lenRaw := 1 + // network size
	//	HeightSize+
	//	TimestampSize +
	//	HashSize +
	//	HashSize + // unknown length
	//	HashSize +
	//	NonceSize
	raw := make([]byte, 121)

	raw[0] = byte(x.Header.GetNetwork())
	binary.LittleEndian.PutUint64(raw[1:], x.Header.GetHeight())
	binary.LittleEndian.PutUint64(raw[9:17], uint64(x.Header.GetTimestamp()))
	copy(raw[17:49], x.Header.GetPrevBlockHash())
	copy(raw[49:81], x.Header.GetTrieHash())
	copy(raw[81:113], utils.ReverseBytes(x.Header.GetDifficulty())) // uint256

	if nonce == nil {
		copy(raw[113:121], x.Header.GetNonce())
	} else {
		copy(raw[113:121], nonce)
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

	randomx.InitCache(cache, x.Header.GetPrevBlockHash())
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
	if txsWithGen[0].Proto.GetType() != ngproto.TxType_GENERATE {
		return fmt.Errorf("first tx shall be a generate")
	}

	for i := 1; i < len(txsWithGen); i++ {
		if txsWithGen[i].Proto.GetType() == ngproto.TxType_GENERATE {
			return fmt.Errorf("except first, other tx shall not be a generate")
		}
	}

	x.Header.TrieHash = NewTxTrie(txsWithGen).TrieRoot()
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
	diff := new(big.Int).SetBytes(x.Header.GetDifficulty())
	target := new(big.Int).Div(MaxTarget, diff)

	if new(big.Int).SetBytes(x.PowHash()).Cmp(target) < 0 {
		return nil
	}

	return fmt.Errorf("block@%d's nonce %x is invalid", x.Header.GetHeight(), x.Header.GetNonce())
}

// GetActualDiff returns the diff decided by nonce.
func (x *Block) GetActualDiff() *big.Int {
	return new(big.Int).Div(MaxTarget, new(big.Int).SetBytes(x.PowHash()))
}

func (x *Block) GetHeader() *ngproto.BlockHeader {
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

	if len(x.Header.GetPrevBlockHash()) != HashSize {
		return fmt.Errorf("block%d's PrevBlockHash length is incorrect", x.Header.GetHeight())
	}

	if len(x.Header.GetTrieHash()) != HashSize {
		return fmt.Errorf("block%d's TrieHash length is incorrect", x.Header.GetHeight())
	}

	if len(x.Header.GetNonce()) != NonceSize {
		return fmt.Errorf("block%d's Nonce length is incorrect", x.Header.GetHeight())
	}

	if x.Header.GetTimestamp() > time.Now().Unix() {
		return fmt.Errorf("block%d's timestamp %d is invalid", x.Header.GetHeight(), x.Header.GetTimestamp())
	}

	if !x.IsSealed() {
		return fmt.Errorf("block@%d has not sealed with nonce", x.Header.GetHeight())
	}

	if !bytes.Equal(NewTxTrie(x.Txs).TrieRoot(), x.Header.GetTrieHash()) {
		return fmt.Errorf("the merkle tree in block@%d is invalid", x.Header.GetHeight())
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

func (x *Block) verifyHash() error {
	if x.Hash == nil {
		x.GetHash()
		return nil
	}

	// re-calc the hash
	raw, err := x.MarshalHeader()
	if err != nil {
		panic(err)
	}

	hash := sha3.Sum256(raw)

	if !bytes.Equal(hash[:], x.Hash) {
		return fmt.Errorf("block@%d hash %x not match its hash %x", x.Header.GetHeight(), hash, x.Hash)
	}

	return nil
}

// GetHash will help you get the hash of block.
func (x *Block) GetHash() []byte {
	if x.Hash == nil {
		raw, err := x.MarshalHeader()
		if err != nil {
			panic(err)
		}

		hash := sha3.Sum256(raw)

		x.Hash = hash[:]
	}

	return x.Hash
}

// GetPrevHash is a helper to get the prev block hash from block header.
func (x *Block) GetPrevHash() []byte {
	return x.Header.GetPrevBlockHash()
}
