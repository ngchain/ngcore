package ngblocks

import (
	"fmt"
	"github.com/c0mm4nd/rlp"

	"github.com/dgraph-io/badger/v3"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// PutNewBlock puts a new block into db and updates the tags.
// should check block before putting
// dev should continue upgrading the state after PutNewBlock
func PutNewBlock(txn *badger.Txn, block *ngtypes.Block) error {
	if block == nil {
		return fmt.Errorf("block is nil")
	}

	hash := block.GetHash()

	err := checkBlock(txn, block.Header.Height, block.Header.PrevBlockHash)
	if err != nil {
		return err
	}

	log.Infof("putting block@%d: %x", block.Header.Height, hash)
	err = putBlock(txn, hash, block)
	if err != nil {
		return err
	}

	// put txs
	err = putTxs(txn, block)
	if err != nil {
		return err
	}

	// update helper
	err = putLatestTags(txn, block.Header.Height, hash)
	if err != nil {
		return err
	}

	return nil
}

func putTxs(txn *badger.Txn, block *ngtypes.Block) error {
	for i := range block.Txs {
		hash := block.Txs[i].GetHash()

		raw, err := rlp.EncodeToBytes(block.Txs[i])
		if err != nil {
			return err
		}

		err = txn.Set(append(txPrefix, hash...), raw)
		if err != nil {
			return err
		}
	}

	return nil
}

func putBlock(txn *badger.Txn, hash []byte, block *ngtypes.Block) error {
	raw, err := rlp.EncodeToBytes(block)
	if err != nil {
		return err
	}

	// put block hash & height
	err = txn.Set(append(blockPrefix, hash...), raw)
	if err != nil {
		return err
	}
	err = txn.Set(append(blockPrefix, utils.PackUint64LE(block.Header.Height)...), hash)
	if err != nil {
		return err
	}

	return nil
}

func putLatestTags(txn *badger.Txn, height uint64, hash []byte) error {
	err := txn.Set(append(blockPrefix, latestHeightTag...), utils.PackUint64LE(height))
	if err != nil {
		return err
	}
	err = txn.Set(append(blockPrefix, latestHashTag...), hash)
	if err != nil {
		return err
	}

	return nil
}
