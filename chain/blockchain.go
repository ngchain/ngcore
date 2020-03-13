package chain

import (
	"bytes"
	"errors"
	"github.com/gogo/protobuf/proto"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/whyrusleeping/go-logging"
	"go.etcd.io/bbolt"
	"sync"
)

var log = logging.MustGetLogger("chain")

type BlockChain struct {
	mu  sync.Mutex
	Mem *MemChain
	DB  *StorageChain
}

func NewBlockChain(db *bbolt.DB) *BlockChain {
	sc := NewStorageChain([]byte("block"), db, ngtypes.GetGenesisBlock())
	latestBlock, err := sc.GetLatestItem(new(ngtypes.Block))
	if err != nil {
		log.Panic(err)
	}
	mem := NewMemChain(latestBlock.(*ngtypes.Block))
	return &BlockChain{
		Mem: mem,
		DB:  sc,
	}
}

func (bc *BlockChain) GetBlockByHash(hash []byte) *ngtypes.Block {
	item, err := bc.Mem.GetItemByHash(hash)
	if err == nil && item != nil {
		return item.(*ngtypes.Block)
	}
	log.Error(err)

	item, err = bc.DB.GetItemByHash(hash, new(ngtypes.Block))
	if err == nil && item != nil {
		return item.(*ngtypes.Block)
	}
	log.Error(err)

	return nil
}

func (bc *BlockChain) GetBlockByHeight(height uint64) *ngtypes.Block {
	item, err := bc.Mem.GetItemByHeight(height)
	if err == nil && item != nil {
		return item.(*ngtypes.Block)
	}

	item, err = bc.DB.GetItemByHeight(height, new(ngtypes.Block))
	if err == nil && item != nil {
		return item.(*ngtypes.Block)
	}

	log.Error(err)

	return nil
}

func (bc *BlockChain) GetLatestBlock() *ngtypes.Block {
	memItem, err := bc.Mem.GetLatestItem()
	if err != nil {
		log.Error(err)
	}

	dbItem, err := bc.DB.GetLatestItem(new(ngtypes.Block))
	if err != nil {
		log.Error(err)
	}

	if dbItem == nil {
		return memItem.(*ngtypes.Block)
	}

	if memItem == nil {
		return dbItem.(*ngtypes.Block)
	}

	if dbItem.GetHeight() >= memItem.GetHeight() {
		return dbItem.(*ngtypes.Block)
	} else {
		return memItem.(*ngtypes.Block)
	}
}

func (bc *BlockChain) GetLatestBlockHash() []byte {
	hash := bc.Mem.GetLatestItemHash()
	if hash != nil {
		return hash
	}

	hash, err := bc.DB.GetLatestHash()
	if err == nil && hash != nil {
		return hash
	}

	log.Error(err)

	return nil
}

func (bc *BlockChain) GetLatestBlockHeight() uint64 {
	height := bc.Mem.latestItemHeight
	if height != 0 {
		return height
	}

	height, err := bc.DB.GetLatestHeight()
	if err == nil && height != 0 {
		return height
	}

	log.Error(err)

	return 0
}

// W

func (bc *BlockChain) PutBlock(block *ngtypes.Block) error {
	if block.IsCheckpoint() && bc.GetLatestBlockHeight() < block.Header.Height {
		bc.mu.Lock()
		defer bc.mu.Unlock()
		log.Info("dumping blocks from mem to DB")
		items := bc.Mem.ExportLongestChain(block)
		if len(items) > 0 {
			err := bc.DB.PutItems(items)
			if err != nil {
				log.Info("failed to put items:", items, err)
				return err
			}
			//bc.Mem.ReleaseMap(block)
		} else {
			return errors.New("chain malformed")
		}
	}

	err := bc.Mem.PutItem(block)
	if err != nil {
		return err
	}

	return nil
}

// VerifyBlockInChain will check a block is valid or not
// - 0. check whether the data integrity
// - 1. check whether the data is correct
// Dont trust any input
func (bc *BlockChain) VerifyBlockInChain(b *ngtypes.Block) error {
	if !b.Header.VerifyHash() {
		return ngtypes.ErrBlockHashInvalid
	}

	trieRootHash := ngtypes.NewTxTrie(b.Transactions).TrieRoot()
	if bytes.Compare(trieRootHash, b.Header.TrieHash) != 0 {
		return ngtypes.ErrBlockMTreeInvalid
	}

	return nil
}

// VerifyChain is a healthy check for whole BlockChain
func (bc *BlockChain) VerifyChain() error {
	err := bc.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("block"))
		lastHash := bucket.Get([]byte(LatestHashTag))
		blockData := bucket.Get(lastHash)
		var beginningBlock, latestBlock ngtypes.Block
		err := proto.Unmarshal(blockData, &latestBlock)
		if err != nil {
			log.Error(err)
		}
		err = latestBlock.CheckError()
		if err != nil {
			log.Error(err)
		}

		lastBlock := beginningBlock

		for i := 0; true; i++ {
			blockData := bucket.Get(lastBlock.Header.PrevBlockHash)
			if blockData == nil {
				if i < 100 {
					err = errors.New("chain is not long enough, a mature chain should have more than 100 blocks")
					return err
				}

				if bytes.Compare(latestBlock.Header.PrevVaultHash, beginningBlock.Header.PrevVaultHash) == 0 {
					err = errors.New("chain is not long enough, a mature chain should have more than 100 blocks")
					return err
				}
			}

			err := proto.Unmarshal(blockData, &beginningBlock)
			if err != nil {
				log.Error(err)
			}
			err = beginningBlock.CheckError()
		}

		return err
	})

	return err
}
