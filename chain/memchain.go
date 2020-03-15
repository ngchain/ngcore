package chain

import (
	"encoding/hex"
	"fmt"
	"github.com/ngin-network/ngcore/ngtypes"
	"strings"
	"sync"
)

func NewMemChain() *MemChain {
	mem := &MemChain{
		BlockHashMap:   make(map[string]*ngtypes.Block),
		BlockHeightMap: make(map[uint64][]string),
		VaultHashMap:   make(map[string]*ngtypes.Vault),
		VaultHeightMap: make(map[uint64][]string),
	}

	return mem
}

func (mc *MemChain) Init(initCheckpoint *ngtypes.Block, initVault *ngtypes.Vault) {
	hash, err := initCheckpoint.CalculateHash()
	if err != nil {
		panic(err)
	}

	mc.BlockHashMap[hex.EncodeToString(hash)] = initCheckpoint
	hashes, ok := mc.BlockHeightMap[initCheckpoint.GetHeight()]
	if ok {
		mc.BlockHeightMap[initCheckpoint.GetHeight()] = append(hashes, hex.EncodeToString(hash))
	} else {
		mc.BlockHeightMap[initCheckpoint.GetHeight()] = []string{hex.EncodeToString(hash)}
	}

	hash, err = initVault.CalculateHash()
	if err != nil {
		panic(err)
	}

	mc.VaultHashMap[hex.EncodeToString(hash)] = initVault
	hashes, ok = mc.VaultHeightMap[initVault.GetHeight()]
	if ok {
		mc.VaultHeightMap[initVault.GetHeight()] = append(hashes, hex.EncodeToString(hash))
	} else {
		mc.VaultHeightMap[initVault.GetHeight()] = []string{hex.EncodeToString(hash)}
	}

}

type MemChain struct {
	sync.RWMutex

	BlockHashMap   map[string]*ngtypes.Block
	BlockHeightMap map[uint64][]string

	VaultHashMap   map[string]*ngtypes.Vault
	VaultHeightMap map[uint64][]string
}

func (mc *MemChain) PutItems(items ...Item) error {
	mc.Lock()
	defer mc.Unlock()

	for i := 0; i < len(items); i++ {
		switch item := items[i].(type) {
		case *ngtypes.Block:
			hash, err := item.CalculateHash()
			if err != nil {
				return err
			}
			mc.BlockHashMap[hex.EncodeToString(hash)] = item
			hashes, ok := mc.BlockHeightMap[item.GetHeight()]
			if ok {
				mc.BlockHeightMap[item.GetHeight()] = append(hashes, hex.EncodeToString(hash))
			} else {
				mc.BlockHeightMap[item.GetHeight()] = []string{hex.EncodeToString(hash)}
			}
		case *ngtypes.Vault:
			hash, err := item.CalculateHash()
			if err != nil {
				return err
			}
			mc.VaultHashMap[hex.EncodeToString(hash)] = item
			hashes, ok := mc.VaultHeightMap[item.GetHeight()]
			if ok {
				mc.VaultHeightMap[item.GetHeight()] = append(hashes, hex.EncodeToString(hash))
			} else {
				mc.VaultHeightMap[item.GetHeight()] = []string{hex.EncodeToString(hash)}
			}
		default:
			panic(fmt.Sprintf("unknown type in chain: %v", item))
		}

	}

	return nil
}

func (mc *MemChain) GetBlockByHash(hash []byte) (*ngtypes.Block, error) {
	mc.RLock()
	defer mc.RUnlock()
	if item, ok := mc.BlockHashMap[hex.EncodeToString(hash)]; !ok {
		return nil, fmt.Errorf("`cannot find the block by hash:%x", hash)
	} else {
		return item, nil
	}
}

func (mc *MemChain) GetBlockByHeight(height uint64) (*ngtypes.Block, error) {
	mc.RLock()
	defer mc.RUnlock()

	if height == 0 {
		return ngtypes.GetGenesisBlock(), nil
	}

	hashes, ok := mc.BlockHeightMap[height]
	if !ok || len(hashes) == 0 {
		return nil, fmt.Errorf("cannot find the block@%d", height)
	}

	for i := 0; i < len(hashes); i++ {
		if item, ok := mc.BlockHashMap[hashes[i]]; ok {
			return item, nil
		}
	}

	return nil, fmt.Errorf("cannot find the block@%d", height)
}

func (mc *MemChain) GetVaultByHash(hash []byte) (*ngtypes.Vault, error) {
	mc.RLock()
	defer mc.RUnlock()
	if item, ok := mc.VaultHashMap[hex.EncodeToString(hash)]; !ok {
		return nil, fmt.Errorf("cannot find the vault by hash:%x", hash)
	} else {
		return item, nil
	}
}

func (mc *MemChain) GetVaultByHeight(height uint64) (*ngtypes.Vault, error) {
	mc.RLock()
	defer mc.RUnlock()

	if height == 0 {
		return ngtypes.GetGenesisVault(), nil
	}

	hashes, ok := mc.VaultHeightMap[height]
	if !ok || len(hashes) == 0 {
		return nil, fmt.Errorf("cannot find the vault@%d in memchain", height)
	}

	for i := 0; i < len(hashes); i++ {
		if item, ok := mc.VaultHashMap[hashes[i]]; ok {
			return item, nil
		}
	}

	return nil, fmt.Errorf("cannot find the vault@%d memchain", height)
}

func (mc *MemChain) GetLatestBlock() (*ngtypes.Block, error) {
	mc.RLock()
	defer mc.RUnlock()
	return mc.GetBlockByHeight(mc.GetLatestBlockHeight())
}

func (mc *MemChain) GetLatestBlockHash() []byte {
	mc.RLock()
	defer mc.RUnlock()

	item, err := mc.GetBlockByHeight(mc.GetLatestBlockHeight())
	if err != nil {
		return nil
	}
	hash, err := item.CalculateHash()
	if err != nil {
		return nil
	}
	return hash
}

func (mc *MemChain) GetLatestBlockHeight() uint64 {
	mc.RLock()
	defer mc.RUnlock()

	var top uint64
	for height, hashes := range mc.BlockHeightMap {
		if hashes != nil && top < height {
			top = height
		}
	}
	return top
}

func (mc *MemChain) GetLatestVault() (*ngtypes.Vault, error) {
	mc.RLock()
	defer mc.RUnlock()

	return mc.GetVaultByHeight(mc.GetLatestVaultHeight())
}

func (mc *MemChain) GetLatestVaultHash() []byte {
	mc.RLock()
	defer mc.RUnlock()

	item, err := mc.GetVaultByHeight(mc.GetLatestVaultHeight())
	if err != nil {
		return nil
	}
	hash, err := item.CalculateHash()
	if err != nil {
		return nil
	}
	return hash
}

func (mc *MemChain) GetLatestVaultHeight() uint64 {
	mc.RLock()
	defer mc.RUnlock()

	var top uint64
	for height, hashes := range mc.VaultHeightMap {
		if hashes != nil && top < height {
			top = height
		}
	}
	return top
}

// TODO
func (mc *MemChain) DelItems(items ...Item) error {
	mc.Lock()
	defer mc.Unlock()

	for i := 0; i < len(items); i++ {
		if items[i] == nil {
			continue
		}
		hash, err := items[i].CalculateHash()
		if err != nil {
			return err
		}
		delete(mc.BlockHashMap, hex.EncodeToString(hash))
		hashes, exists := mc.BlockHeightMap[items[i].GetHeight()]
		if !exists {
			return fmt.Errorf("cannot find height %d", items[i].GetHeight())
		}
		for i := range hashes {
			if strings.Compare(hashes[i], hex.EncodeToString(hash)) == 0 {
				hashes = append(hashes[:i], hashes[i+1:]...)
				break
			}
		}
		mc.BlockHeightMap[items[i].GetHeight()] = hashes
	}
	return nil
}

// ExportAllItem exports all items and then should save them all into storageChain
func (mc *MemChain) ExportLongestChain(end *ngtypes.Block, maxLen int) []Item {
	mc.RLock()
	defer mc.RUnlock()

	if (end.GetHeight()+1)%ngtypes.BlockCheckRound != 0 {
		return nil
	}

	// fetch (lastCP, CP]
	items := make([]Item, 0, len(mc.BlockHashMap)+len(mc.VaultHashMap))
	cur := end
	items = append(items, cur)

	for i := 0; i < maxLen; i++ {
		item, ok := mc.BlockHashMap[hex.EncodeToString(cur.GetPrevHash())]
		if !ok {
			log.Errorf("this chain lacks block: %x@%d", cur.GetPrevHash(), cur.GetHeight()-1)
			return nil
		}
		items = append(items, item)
		if cur.IsCheckpoint() {
			if v, ok := mc.VaultHashMap[hex.EncodeToString(cur.Header.PrevVaultHash)]; ok {
				items = append(items, v)
			}
		}

		cur = item
	}

	//err := mc.DelItems(items...)
	//if err != nil {
	//	log.Error(err)
	//}

	// reverse the slice
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}

	if len(items) > 0 {
		log.Infof("exported chain from %d to %d", items[0].GetHeight(), items[len(items)-1].GetHeight())
	}

	return items
}
