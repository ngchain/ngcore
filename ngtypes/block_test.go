package ngtypes

import (
	"bytes"
	"fmt"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/NebulousLabs/fastrand"
	"github.com/gogo/protobuf/proto"

	"github.com/mr-tron/base58"
	"github.com/ngin-network/cryptonight-go"
)

func TestBlock_GetHash(t *testing.T) {
	b := GetGenesisBlock()
	block := b.Header.CalculateHash()
	t.Log(len(block))
}

func TestGetGenesisBlockNonce(t *testing.T) {
	// new genesisBlock
	runtime.GOMAXPROCS(3)

	b := NewBareBlock(2, nil, nil, GenesisTarget)
	b, err := b.ToUnsealing(nil)
	if err != nil {
		log.Fatal(err)
	}

	genesisTarget := new(big.Int).SetBytes(b.Header.Target)

	nCh := make(chan []byte, 1)
	stopCh := make(chan struct{}, 1)
	thread := 3

	for i := 0; i < thread; i++ {
		go calcHash(i, b, genesisTarget, nCh, stopCh)
	}

	answer := <-nCh
	stopCh <- struct{}{}
	blob := b.Header.GetPoWBlob(answer)
	if err != nil {
		log.Panic(err)
	}
	hash := cryptonight.Sum(blob, 0)
	fmt.Println("N is ", answer, " Hash is ", base58.FastBase58Encoding(hash))
}

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
			blob := b.Header.GetPoWBlob(random)

			hash := cryptonight.Sum(blob, 0)
			//fmt.Println(new(big.Int).SetBytes(hash).Uint64())
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

func TestBlock_Marshal(t *testing.T) {
	block, _ := GetGenesisBlock().Marshal()

	var genesisBlock Block
	_ = proto.Unmarshal(block, &genesisBlock)
	_block, _ := genesisBlock.Marshal()
	if !bytes.Equal(block, _block) {
		t.Fail()
	}
}

func TestGetGenesisBlock(t *testing.T) {
	hash := GetGenesisBlock().HeaderHash

	d, _ := GetGenesisBlock().Marshal()
	log.Infof("GenesisBlock hex: %x", d)
	log.Infof("GenesisBlock hash: %x", hash)
}
