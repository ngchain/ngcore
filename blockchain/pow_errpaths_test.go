package blockchain_test

import (
	"testing"

	"github.com/c0mm4nd/rlp"
	"go.etcd.io/bbolt"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/storage"
	"github.com/ngchain/ngcore/utils"
)

// CheckHealth panics when a canonical block between the origin and the tip
// cannot be loaded (the height index points at a missing block).
func TestCheckHealthPanicsOnMissingBlock(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	b1 := mineBlock(t, ngtypes.GetGenesisBlock(ngtypes.ZERONET), miner)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}

	// remove the block data behind height 1 while keeping the latest tags,
	// so the health walk fails to load block@1
	if err := chain.DB.Update(func(txn *bbolt.Tx) error {
		return txn.Bucket(storage.BlockBucketName).Delete(b1.GetHash())
	}); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("CheckHealth must panic when a canonical block is missing")
		}
	}()

	chain.CheckHealth(ngtypes.ZERONET)
}

// CheckHealth panics when the prev-hash linkage of a canonical block is
// broken (the stored block at a height does not chain onto the previous one).
func TestCheckHealthPanicsOnBrokenLink(t *testing.T) {
	chain := newTestChain(t)
	miner, _ := ngtypes.GenerateKey()

	genesis := ngtypes.GetGenesisBlock(ngtypes.ZERONET)
	b1 := mineBlock(t, genesis, miner)
	b2 := mineBlock(t, b1, miner)
	if err := chain.ApplyBlock(b1); err != nil {
		t.Fatal(err)
	}
	if err := chain.ApplyBlock(b2); err != nil {
		t.Fatal(err)
	}

	// forge a block at height 1 whose hash differs from b1, so b2's prev
	// hash no longer matches the block sitting at height 1
	forged := mineBlock(t, genesis, func() *ngtypes.PrivateKey {
		k, _ := ngtypes.GenerateKey()
		return k
	}())
	if err := chain.DB.Update(func(txn *bbolt.Tx) error {
		bb := txn.Bucket(storage.BlockBucketName)
		raw, encErr := rlp.EncodeToBytes(forged)
		if encErr != nil {
			return encErr
		}
		if err := bb.Put(forged.GetHash(), raw); err != nil {
			return err
		}
		return bb.Put(utils.PackUint64LE(1), forged.GetHash())
	}); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("CheckHealth must panic on a broken prev-hash link")
		}
	}()

	chain.CheckHealth(ngtypes.ZERONET)
}

// GetTxLocation surfaces the error when a tx is indexed to a block hash whose
// block is not stored (a dangling tx->block index entry).
func TestGetTxLocationDanglingBlock(t *testing.T) {
	chain := newTestChain(t)

	txHash := make([]byte, 32)
	txHash[0] = 0x01
	danglingBlock := make([]byte, 32)
	danglingBlock[0] = 0x02

	if err := chain.DB.Update(func(txn *bbolt.Tx) error {
		txBucket := txn.Bucket(storage.TxBucketName)
		// index the tx to a block hash that is not stored
		return txBucket.Put(append(append([]byte{}, storage.TxBlockPrefix...), txHash...), danglingBlock)
	}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := chain.GetTxLocation(txHash); err == nil {
		t.Fatal("GetTxLocation must fail when the indexed block is missing")
	}
}
