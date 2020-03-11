package chain

import (
	"bytes"
	"encoding/hex"
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

		LatestItemHeight: 0,
		LatestItemHash:   nil,
	}
	err := mem.PutItem(initCheckpint)
	if err != nil {
		log.Panic(err)
	}

	return mem
}

type MemChain struct {
	HashMap   *sync.Map
	HeightMap *sync.Map

	LatestItemHeight uint64
	LatestItemHash   []byte
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

	mc.LatestItemHeight = height
	mc.LatestItemHash = hash

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
	return mc.GetItemByHeight(mc.LatestItemHeight)
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
				return nil
			}
			cur = item.(Item)
		}
	}

	log.Infof("dumped chain from %d to %d", items[0].GetHeight(), items[len(items)-1].GetHeight())
	return items
}
