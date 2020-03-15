package chain

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/ngin-network/ngcore/utils"
	"go.etcd.io/bbolt"
)

var blockBucketName = []byte("block")
var vaultBucketName = []byte("vault")

type storageChain struct {
	*bbolt.DB
}

func NewStorageChain(db *bbolt.DB) *storageChain {
	c := &storageChain{
		DB: db,
	}

	if c.IsNewChain() {
		c.initBuckets()
	}
	return c
}

func (c *storageChain) initBuckets() {
	c.DB.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(blockBucketName))
		if err != nil {
			panic(err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte(vaultBucketName))
		if err != nil {
			panic(err)
		}
		return nil
	})
}

func (c *storageChain) Init(initCheckpoint *ngtypes.Block, initVault *ngtypes.Vault) {
	c.DB.Update(func(tx *bbolt.Tx) error {
		blockBucket := tx.Bucket([]byte(blockBucketName))
		vaultBucket := tx.Bucket([]byte(vaultBucketName))

		hash, _ := initCheckpoint.CalculateHash()
		raw, _ := initCheckpoint.Marshal()
		blockBucket.Put(hash, raw)
		blockBucket.Put(utils.PackUint64LE(initCheckpoint.GetHeight()), raw)
		blockBucket.Put([]byte(LatestHashTag), hash)
		blockBucket.Put([]byte(LatestHeightTag), utils.PackUint64LE(initCheckpoint.GetHeight()))

		hash, _ = initVault.CalculateHash()
		raw, _ = initVault.Marshal()
		vaultBucket.Put(hash, raw)
		vaultBucket.Put(utils.PackUint64LE(initVault.GetHeight()), raw)
		vaultBucket.Put([]byte(LatestHashTag), hash)
		vaultBucket.Put([]byte(LatestHeightTag), utils.PackUint64LE(initVault.GetHeight()))

		return nil
	})

}

func (c *storageChain) IsNewChain() bool {
	tx, err := c.Begin(false)
	if err != nil {
		log.Panic(err)
	}
	defer tx.Rollback()

	return tx.Bucket(blockBucketName) == nil || tx.Bucket(vaultBucketName) == nil
}

func (c *storageChain) PutChain(chain ...Item) error {
	err := c.DB.Update(func(tx *bbolt.Tx) error {
		blockBucket := tx.Bucket(blockBucketName)
		vaultBucket := tx.Bucket(vaultBucketName)

		for i := 0; i < len(chain); i++ {
			switch item := chain[i].(type) {
			case *ngtypes.Block:
				log.Infof("putting block@%d", item.GetHeight())
				if item.IsCheckpoint() && vaultBucket.Get(utils.PackUint64LE(item.GetHeight()/ngtypes.BlockCheckRound)) == nil { // noBaseVaultInHeight(height)
					return fmt.Errorf("cannot put block@%d due to no vault@%d gen on this checkpoint, the vault should be put before it)", item.GetHeight(), item.GetHeight()/ngtypes.BlockCheckRound)
				}

				hash, _ := item.CalculateHash()
				raw, _ := item.Marshal()
				err := blockBucket.Put(hash, raw)
				if err != nil {
					return err
				}
				err = blockBucket.Put(utils.PackUint64LE(item.GetHeight()), raw)
				if err != nil {
					return err
				}
				err = blockBucket.Put([]byte(LatestHashTag), hash)
				if err != nil {
					return err
				}
				err = blockBucket.Put([]byte(LatestHeightTag), utils.PackUint64LE(item.GetHeight()))
				if err != nil {
					return err
				}
			case *ngtypes.Vault:
				log.Infof("putting vault@%d", item.GetHeight())
				latestBlockHeightRaw := make([]byte, 8)
				copy(latestBlockHeightRaw, blockBucket.Get([]byte(LatestHeightTag)))
				if (binary.LittleEndian.Uint64(latestBlockHeightRaw)+1)%ngtypes.BlockCheckRound != 0 {
					return fmt.Errorf("failed putting vault@%d: cannot put the vault after @%d", item.GetHeight(), binary.LittleEndian.Uint64(blockBucket.Get([]byte(LatestHeightTag))))
				}
				hash, _ := item.CalculateHash()
				raw, _ := item.Marshal()
				err := vaultBucket.Put(hash, raw)
				if err != nil {
					return err
				}
				err = vaultBucket.Put(utils.PackUint64LE(item.GetHeight()), raw)
				if err != nil {
					return err
				}
				err = vaultBucket.Put([]byte(LatestHashTag), hash)
				if err != nil {
					return err
				}
				err = vaultBucket.Put([]byte(LatestHeightTag), utils.PackUint64LE(item.GetHeight()))
				if err != nil {
					return err
				}
			default:
				panic(fmt.Sprintf("unknown type in chain: %v", item))
			}
		}

		return nil
	})

	return err
}

func (c *storageChain) GetVaultByHash(hash []byte) (*ngtypes.Vault, error) {
	var vault ngtypes.Vault
	err := c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(vaultBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		raw := bucket.Get(hash)
		if raw == nil {
			return fmt.Errorf("cannot find vault by hash: %x", hash)
		}

		err := proto.Unmarshal(raw, &vault)
		if err != nil {
			return err
		}

		return nil
	})

	return &vault, err
}

func (c *storageChain) GetBlockByHash(hash []byte) (*ngtypes.Block, error) {
	var block ngtypes.Block
	err := c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(blockBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		raw := bucket.Get(hash)
		if raw == nil {
			return fmt.Errorf("cannot find block by hash: %x", hash)
		}

		err := proto.Unmarshal(raw, &block)
		if err != nil {
			return err
		}

		return nil
	})

	return &block, err
}

func (c *storageChain) GetBlockByHeight(height uint64) (*ngtypes.Block, error) {
	var block ngtypes.Block
	err := c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(blockBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		hash := bucket.Get(utils.PackUint64LE(height))
		if hash == nil {
			return fmt.Errorf("cannot find the block@%d", height)
		}

		raw := bucket.Get(hash)
		if raw == nil {
			return fmt.Errorf("cannot find block@%d", height)
		}

		err := proto.Unmarshal(raw, &block)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &block, nil
}

func (c *storageChain) GetVaultByHeight(height uint64) (*ngtypes.Vault, error) {
	var vault ngtypes.Vault
	err := c.View(func(tx *bbolt.Tx) error {

		bucket := tx.Bucket(blockBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		hash := bucket.Get(utils.PackUint64LE(height))
		if hash == nil {
			return fmt.Errorf("cannot find the vault@%d", height)
		}

		raw := bucket.Get(hash)
		if raw == nil {
			return fmt.Errorf("cannot find vault@%x", height)
		}

		err := proto.Unmarshal(raw, &vault)
		if err != nil {
			return err
		}

		return nil
	})

	return &vault, err
}

func (c *storageChain) GetLatestBlockHash() ([]byte, error) {
	var hash = make([]byte, 32)

	err := c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(blockBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		hashInDB := bucket.Get([]byte(LatestHashTag))
		if hashInDB == nil {
			return fmt.Errorf("no block hash at latest hash tag")
		}

		copy(hash, hashInDB)
		return nil
	})

	return hash, err
}

func (c *storageChain) GetLatestVaultHash() ([]byte, error) {
	var hash = make([]byte, 32)

	err := c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(vaultBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		hashInDB := bucket.Get([]byte(LatestHashTag))
		if hashInDB == nil {
			return fmt.Errorf("no vault hash at latest hash tag")
		}

		copy(hash, hashInDB)
		return nil
	})

	return hash, err
}

func (c *storageChain) GetLatestBlockHeight() (uint64, error) {
	var height uint64

	err := c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(blockBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		heightInDB := bucket.Get([]byte(LatestHeightTag))
		if heightInDB == nil {
			return fmt.Errorf("no block height data in latest height tag")
		}
		height = binary.LittleEndian.Uint64(heightInDB)
		return nil
	})

	return height, err
}

func (c *storageChain) GetLatestVaultHeight() (uint64, error) {
	var height uint64

	err := c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(vaultBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		heightInDB := bucket.Get([]byte(LatestHeightTag))
		if heightInDB == nil {
			return fmt.Errorf("no vault height data in latest height tag")
		}
		height = binary.LittleEndian.Uint64(heightInDB)
		return nil
	})

	return height, err
}

func (c *storageChain) GetLatestBlock() (*ngtypes.Block, error) {
	var block ngtypes.Block
	err := c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(blockBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		latestHashInDB := bucket.Get([]byte(LatestHashTag))
		if latestHashInDB == nil {
			return fmt.Errorf("no block hash at latest hash tag")
		}

		raw := bucket.Get(latestHashInDB)
		if raw == nil {
			return fmt.Errorf("cannot find the latest block by latestTag")
		}

		err := proto.Unmarshal(raw, &block)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &block, nil
}

func (c *storageChain) GetLatestVault() (*ngtypes.Vault, error) {
	var vault ngtypes.Vault
	err := c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(vaultBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		latestHashInDB := bucket.Get([]byte(LatestHashTag))
		if latestHashInDB == nil {
			return fmt.Errorf("no vault hash at latest hash tag")
		}

		raw := bucket.Get(latestHashInDB)
		if raw == nil {
			return fmt.Errorf("cannot find the latest vault by latestTag")
		}

		err := vault.Unmarshal(raw)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &vault, nil
}

func (c *storageChain) DumpAllBlocksByHash() map[string]*ngtypes.Block {
	kv := make(map[string]*ngtypes.Block)
	c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(blockBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		_ = bucket.ForEach(func(k, v []byte) error {
			if len(k) == 32 {
				var block ngtypes.Block
				proto.Unmarshal(v, &block)
				kv[hex.EncodeToString(k)] = &block
			}
			return nil
		})

		return nil
	})
	return kv
}

func (c *storageChain) DumpAllVaultsByHash() map[string]*ngtypes.Vault {
	kv := make(map[string]*ngtypes.Vault)
	c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(vaultBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		_ = bucket.ForEach(func(k, v []byte) error {
			var vault ngtypes.Vault
			proto.Unmarshal(v, &vault)
			kv[hex.EncodeToString(k)] = &vault
			return nil
		})

		return nil
	})
	return kv
}

func (c *storageChain) DumpAllBlocksByHeight() map[uint64]*ngtypes.Block {
	kv := make(map[uint64]*ngtypes.Block)
	c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(blockBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		_ = bucket.ForEach(func(k, v []byte) error {
			if len(k) == 8 {
				var block ngtypes.Block
				proto.Unmarshal(v, &block)
				kv[binary.LittleEndian.Uint64(k)] = &block
			}
			return nil
		})

		return nil
	})
	return kv
}

func (c *storageChain) DumpAllVaultsByHeight() map[uint64]*ngtypes.Vault {
	kv := make(map[uint64]*ngtypes.Vault)
	c.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(vaultBucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		_ = bucket.ForEach(func(k, v []byte) error {
			if len(k) == 8 {
				var vault ngtypes.Vault
				proto.Unmarshal(v, &vault)
				kv[binary.LittleEndian.Uint64(k)] = &vault
			}
			return nil
		})

		return nil
	})
	return kv
}
