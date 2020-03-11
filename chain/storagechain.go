package chain

import (
	"encoding/binary"
	"encoding/hex"
	"github.com/ngin-network/ngcore/utils"
	"go.etcd.io/bbolt"
)

func NewStorageChain(name []byte, db *bbolt.DB, genesisItem Item) *StorageChain {
	sc := &StorageChain{
		BucketName: name,
		DB:         db,
	}

	if sc.IsNewChain() {
		sc.Init(genesisItem)
	}

	//var err error
	//sc.LatestItemHash, err = sc.GetLatestHash()
	//if err != nil {
	//	log.Panic(err)
	//}
	//
	//sc.LatestItemHeight, err = sc.GetLatestHeight()
	//if err != nil {
	//	log.Panic(err)
	//}

	return sc
}

type StorageChain struct {
	*bbolt.DB
	BucketName []byte
}

func (sc *StorageChain) IsNewChain() bool {
	tx, err := sc.Begin(false)
	if err != nil {
		log.Panic(err)
	}
	defer tx.Rollback()

	return tx.Bucket(sc.BucketName) == nil
}

func (sc *StorageChain) Init(genesisItem Item) {
	sc.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket(sc.BucketName)
		if err != nil {
			log.Panic(err)
		}

		bucket := tx.Bucket(sc.BucketName)
		if bucket == nil {
			log.Panic(bbolt.ErrBucketNotFound)
			return nil
		}

		height := genesisItem.GetHeight()
		if bucket.Get(utils.PackUint64LE(height)) != nil {
			log.Panic(ErrItemHashInSameHeight)
		}

		hash, err := genesisItem.CalculateHash()
		if err != nil {
			log.Panic(err)
		}

		raw, err := genesisItem.Marshal()
		if err != nil {
			log.Panic(err)
		}

		err = bucket.Put(hash, raw)
		if err != nil {
			log.Panic(err)
		}

		err = bucket.Put([]byte(LatestHeightTag), utils.PackUint64LE(height))
		if err != nil {
			log.Panic(err)
		}

		err = bucket.Put([]byte(LatestHashTag), hash)
		if err != nil {
			log.Panic(err)
		}

		return nil
	})
}

func (sc *StorageChain) PutItem(item Item) error {
	err := sc.Update(func(tx *bbolt.Tx) error {

		bucket := tx.Bucket(sc.BucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		height := item.GetHeight()
		if bucket.Get(utils.PackUint64LE(height)) != nil {
			return ErrItemHashInSameHeight
		}

		hash, err := item.CalculateHash()
		if err != nil {
			return err
		}

		raw, err := item.Marshal()
		if err != nil {
			return err
		}

		err = bucket.Put(hash, raw)
		if err != nil {
			return err
		}

		err = bucket.Put(utils.PackUint64LE(height), hash)
		if err != nil {
			return err
		}

		bHeight := make([]byte, 8)
		latestHeightInDB := bucket.Get([]byte(LatestHeightTag))
		copy(bHeight, latestHeightInDB)

		if binary.LittleEndian.Uint64(bHeight) < height {
			err = bucket.Put([]byte(LatestHeightTag), utils.PackUint64LE(height))
			if err != nil {
				return err
			}

			err = bucket.Put([]byte(LatestHashTag), hash)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func (sc *StorageChain) GetItemByHash(hash []byte, itemPlaceholder Item) (Item, error) {
	err := sc.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(sc.BucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		raw := bucket.Get(hash)
		if raw == nil {
			return ErrNoItemInHash
		}

		err := itemPlaceholder.Unmarshal(raw)
		if err != nil {
			return err
		}

		return nil
	})

	return itemPlaceholder, err
}

func (sc *StorageChain) GetItemByHeight(height uint64, itemPlaceholder Item) (Item, error) {
	err := sc.View(func(tx *bbolt.Tx) error {

		bucket := tx.Bucket(sc.BucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		hash := bucket.Get(utils.PackUint64LE(height))
		if hash == nil {
			return ErrNoItemHashInHeight
		}

		raw := bucket.Get(hash)
		if raw == nil {
			return ErrNoItemInHash
		}

		err := itemPlaceholder.Unmarshal(raw)
		if err != nil {
			return err
		}

		return nil
	})

	return itemPlaceholder, err
}

func (sc *StorageChain) GetLatestHash() ([]byte, error) {
	var hash = make([]byte, 32)

	err := sc.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(sc.BucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		hashInDB := bucket.Get([]byte(LatestHashTag))
		if hashInDB == nil {
			return ErrNoHashInTag
		}

		copy(hash, hashInDB)
		return nil
	})

	return hash, err
}

func (sc *StorageChain) GetLatestHeight() (uint64, error) {
	var height uint64

	err := sc.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(sc.BucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		heightInDB := bucket.Get([]byte(LatestHeightTag))
		if heightInDB == nil {
			return ErrNoHeightInTag
		}
		height = binary.LittleEndian.Uint64(heightInDB)
		return nil
	})

	return height, err
}

func (sc *StorageChain) GetLatestItem(itemPlaceholder Item) (Item, error) {
	err := sc.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(sc.BucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		latestHashInDB := bucket.Get([]byte(LatestHashTag))
		if latestHashInDB == nil {
			return ErrNoHashInTag
		}

		raw := bucket.Get(latestHashInDB)
		if raw == nil {
			return ErrNoItemHashInHeight
		}

		err := itemPlaceholder.Unmarshal(raw)
		if err != nil {
			return err
		}
		return nil
	})

	return itemPlaceholder, err
}

func (sc *StorageChain) PutItems(items []Item) error {
	err := sc.Update(func(tx *bbolt.Tx) error {

		bucket := tx.Bucket(sc.BucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		for _, item := range items {
			height := item.GetHeight()
			if bucket.Get(utils.PackUint64LE(height)) != nil {
				return ErrItemHashInSameHeight
			}

			hash, err := item.CalculateHash()
			if err != nil {
				return err
			}

			raw, err := item.Marshal()
			if err != nil {
				return err
			}

			err = bucket.Put(hash, raw)
			if err != nil {
				return err
			}

			err = bucket.Put(utils.PackUint64LE(height), hash)
			if err != nil {
				return err
			}

			bHeight := make([]byte, 8)
			latestHeightInDB := bucket.Get([]byte(LatestHeightTag))
			copy(bHeight, latestHeightInDB)

			if binary.LittleEndian.Uint64(bHeight) < height {
				err = bucket.Put([]byte(LatestHeightTag), utils.PackUint64LE(height))
				if err != nil {
					return err
				}

				err = bucket.Put([]byte(LatestHashTag), hash)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})

	return err
}

func (sc *StorageChain) GetAll() map[string]interface{} {
	kv := make(map[string]interface{})
	sc.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(sc.BucketName)
		if bucket == nil {
			return bbolt.ErrBucketNotFound
		}

		_ = bucket.ForEach(func(k, v []byte) error {
			kv[hex.EncodeToString(k)] = hex.EncodeToString(v)
			return nil
		})

		return nil
	})
	return kv
}
