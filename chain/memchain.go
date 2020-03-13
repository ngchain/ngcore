package chain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/ngin-network/ngcore/ngtypes"
	"sync"
)

func NewMemChain(initCheckpint *ngtypes.Block) *MemChain {
	if !initCheckpint.IsCheckpoint() {
		log.Panic("failed to init mem chain: the init block is not a checkpoint")
	}
	mem := &MemChain{
		HashMap:   &sync.Map{},
		HeightMap: &sync.Map{},

		latestItemHeight: 0,
	}
	//err := mem.PutItem(initCheckpint)
	//if err != nil {
	//	log.Panic(err)
	//}

	height := initCheckpint.GetHeight()

	hash, err := initCheckpint.CalculateHash()
	if err != nil {
		panic(err)
	}

	mem.HashMap.Store(hex.EncodeToString(hash), initCheckpint)
	hashes, ok := mem.HeightMap.Load(initCheckpint.GetHeight())
	if ok {
		mem.HeightMap.Store(initCheckpint.GetHeight(), append(hashes.([][]byte), hash))
	} else {
		mem.HeightMap.Store(initCheckpint.GetHeight(), [][]byte{hash})
	}

	mem.latestItemHeight = height

	return mem
}

type MemChain struct {
	HashMap   *sync.Map
	HeightMap *sync.Map

	latestItemHeight uint64
}

func (mc *MemChain) PutItem(item Item) error {
	height := item.GetHeight()

	hash, err := item.CalculateHash()
	if err != nil {
		return err
	}

	mc.HashMap.Store(hex.EncodeToString(hash), item)
	hashes, ok := mc.HeightMap.Load(item.GetHeight())
	if ok {
		mc.HeightMap.Store(item.GetHeight(), append(hashes.([][]byte), hash))
	} else {
		mc.HeightMap.Store(item.GetHeight(), [][]byte{hash})
	}

	mc.latestItemHeight = height

	return nil
}

func (mc *MemChain) GetItemByHash(hash []byte) (Item, error) {
	if item, ok := mc.HashMap.Load(hex.EncodeToString(hash)); !ok {
		return nil, ErrNoItemInHash
	} else {
		return item.(Item), nil
	}
}

func (mc *MemChain) GetItemByHeight(height uint64) (Item, error) {
	hashes, ok := mc.HeightMap.Load(height)
	if !ok || len(hashes.([][]byte)) == 0 {
		return nil, ErrNoItemInHeight
	}

	for i := 0; i < len(hashes.([][]byte)); i++ {
		if item, ok := mc.HashMap.Load(hex.EncodeToString(hashes.([][]byte)[i])); ok {
			return item.(Item), nil
		}
	}

	return nil, ErrNoItemInHeight
}

func (mc *MemChain) GetLatestItem() (Item, error) {
	return mc.GetItemByHeight(mc.latestItemHeight)
}

func (mc *MemChain) GetLatestItemHash() []byte {
	item, err := mc.GetItemByHeight(mc.latestItemHeight)
	if err != nil {
		panic(err)
	}
	hash, err := item.CalculateHash()
	if err != nil {
		panic(err)
	}
	return hash
}

func (mc *MemChain) GetLatestItemHeight() uint64 {
	return mc.latestItemHeight
}

// ReleaseMap always run after blockchain ExportAllItem, but never after vaultchain's action
func (mc *MemChain) ReleaseMap(checkpoint *ngtypes.Block) {
	mc.HeightMap.Range(func(key, value interface{}) bool {
		height := key.(uint64)
		if height < checkpoint.GetHeight() {
			hashes := value.([][]byte)
			mc.HeightMap.Delete(height)
			for i := range hashes {
				mc.HashMap.Delete(hex.EncodeToString(hashes[i]))
			}
		}

		return true
	})
}

func (mc *MemChain) DelItems(items ...Item) error {
	for i := 0; i < len(items); i++ {
		if items[i] == nil {
			continue
		}
		hash, err := items[i].CalculateHash()
		if err != nil {
			return err
		}
		mc.HashMap.Delete(hex.EncodeToString(hash))
		heights, exists := mc.HeightMap.Load(items[i].GetHeight())
		if !exists {
			return fmt.Errorf("cannot find height %d", items[i].GetHeight())
		}
		h := heights.([][]byte)
		for i := range h {
			if bytes.Compare(h[i], hash) == 0 {
				h = append(h[:i], h[i+1:]...)
				break
			}
		}
		mc.HeightMap.Store(items[i].GetHeight(), heights)
	}
	return nil
}

// ExportAllItem exports all items and then should save them all into storageChain
func (mc *MemChain) ExportLongestChain(checkpoint Item) []Item {
	// fetch (lastCP, CP]
	items := make([]Item, ngtypes.CheckRound)
	cur := checkpoint

	for i := 0; i < ngtypes.CheckRound; i++ {
		items[ngtypes.CheckRound-1-i] = cur

		if bytes.Compare(cur.GetPrevHash(), ngtypes.GenesisBlockHash) != 0 {
			item, ok := mc.HashMap.Load(hex.EncodeToString(cur.GetPrevHash()))
			if !ok {
				log.Errorf("this chain lacks block: %x @ %d", cur.GetPrevHash(), cur.GetHeight()-1)
				err := mc.DelItems(items...)
				if err != nil {
					log.Error(err)
				}
				return nil
			}

			cur = item.(Item)
		}
	}

	err := mc.DelItems(items...)
	log.Error(err)
	log.Infof("dumped chain from %d to %d", items[0].GetHeight(), items[len(items)-1].GetHeight())
	return items
}
