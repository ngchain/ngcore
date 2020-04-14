package ngtypes

import (
	"bytes"
	"fmt"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/NebulousLabs/fastrand"
	"golang.org/x/crypto/sha3"
	"google.golang.org/protobuf/proto"

	"github.com/ngchain/cryptonight-go"

	"github.com/ngchain/ngcore/utils"
)

// TestBlock_GetHash test func GetGenesisBlock() and return Hash value
func TestBlock_GetHash(t *testing.T) {
	b := GetGenesisBlock()
	headerHash := b.CalculateHeaderHash()
	t.Log(len(headerHash))
}

// TestGetGenesisBlockNonce test func NewBareBlock()
func TestGetGenesisBlockNonce(t *testing.T) {
	// new genesisBlock
	runtime.GOMAXPROCS(3)

	b := GetGenesisBlock()
	genesisTarget := new(big.Int).SetBytes(b.Header.Target)

	nCh := make(chan []byte, 1)
	stopCh := make(chan struct{}, 1)
	thread := 3

	for i := 0; i < thread; i++ {
		go calcHash(i, b, genesisTarget, nCh, stopCh)
	}

	answer := <-nCh
	stopCh <- struct{}{}
	fmt.Printf("Genesis Block's Nonce: %x", answer)
}

func TestBlock_IsGenesis(t *testing.T) {
	g := GetGenesisBlock()
	if !g.IsGenesis() {
		t.Fail()
	}
	if err := g.CheckError(); err != nil {
		t.Log(err)
		t.Fail()
	}

	raw, _ := utils.Proto.Marshal(g)
	gg := new(Block)
	_ = utils.Proto.Unmarshal(raw, gg)
	if !gg.IsGenesis() {
		t.Fail()
	}

	if err := gg.CheckError(); err != nil {
		t.Log(err)
		t.Fail()
	}
}

// calcHash get the hash of block
func calcHash(id int, b *Block, target *big.Int, answerCh chan []byte, stopCh chan struct{}) {
	fmt.Println("thread ", id, " running")
	fmt.Println("target is ", target.String())

	t := time.Now()

	for {
		select {
		case <-stopCh:
			return
		default:
			random := fastrand.Bytes(8)
			blob := b.GetPoWBlob(random)

			hash := cryptonight.Sum(blob, 0)
			if new(big.Int).SetBytes(hash).Cmp(target) < 0 {
				answerCh <- random
				fmt.Println("Found ", random, hash)
				elapsed := time.Since(t)
				fmt.Println("Elapsed: ", elapsed)
				return
			}
		}
	}
}

// TestBlock_Marshal test func GetGenesisBlock()'s Marshal()
func TestBlock_Marshal(t *testing.T) {
	block, _ := utils.Proto.Marshal(GetGenesisBlock())

	var genesisBlock Block
	_ = proto.Unmarshal(block, &genesisBlock)
	_block, _ := utils.Proto.Marshal(&genesisBlock)
	if !bytes.Equal(block, _block) {
		t.Fail()
	}
}

// TestGetGenesisBlock test func GetGenesisBlock()'s parameter passing
func TestGetGenesisBlock(t *testing.T) {
	d, _ := utils.Proto.Marshal(GetGenesisBlock())
	hash := sha3.Sum256(d)

	log.Infof("GenesisBlock hex: %x", d)
	log.Infof("GenesisBlock hash: %x", hash)
}
