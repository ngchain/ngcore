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
		switch items[i].(type) {
		case *ngtypes.Block:
			block := items[i].(*ngtypes.Block)
			hash, err := block.CalculateHash()
			if err != nil {
				return err
			}
			mc.BlockHashMap[hex.EncodeToString(hash)] = block
			hashes, ok := mc.BlockHeightMap[block.GetHeight()]
			if ok {
				mc.BlockHeightMap[block.GetHeight()] = append(hashes, hex.EncodeToString(hash))
			} else {
				mc.BlockHeightMap[block.GetHeight()] = []string{hex.EncodeToString(hash)}
			}
		case *ngtypes.Vault:
			vault := items[i].(*ngtypes.Vault)
			hash, err := vault.CalculateHash()
			if err != nil {
				return err
			}
			log.Info(vault.GetHeight(), hex.EncodeToString(hash))
			mc.VaultHashMap[hex.EncodeToString(hash)] = vault
			hashes, ok := mc.VaultHeightMap[vault.GetHeight()]
			if ok {
				mc.VaultHeightMap[vault.GetHeight()] = append(hashes, hex.EncodeToString(hash))
			} else {
				mc.VaultHeightMap[vault.GetHeight()] = []string{hex.EncodeToString(hash)}
			}
		default:
			panic(fmt.Sprintf("unknown type in chain: %v", items[i]))
		}
	}

	return nil
}

func (mc *MemChain) PutBlocks(blocks ...*ngtypes.Block) error {
	mc.Lock()
	defer mc.Unlock()

	for i := 0; i < len(blocks); i++ {
		block := blocks[i]
		hash, err := block.CalculateHash()
		if err != nil {
			return err
		}
		mc.BlockHashMap[hex.EncodeToString(hash)] = block
		hashes, ok := mc.BlockHeightMap[block.GetHeight()]
		if ok {
			mc.BlockHeightMap[block.GetHeight()] = append(hashes, hex.EncodeToString(hash))
		} else {
			mc.BlockHeightMap[block.GetHeight()] = []string{hex.EncodeToString(hash)}
		}
	}

	return nil
}

func (mc *MemChain) PutVaults(vaults ...*ngtypes.Vault) error {
	mc.Lock()
	defer mc.Unlock()

	for i := 0; i < len(vaults); i++ {
		vault := vaults[i]
		hash, err := vault.CalculateHash()
		if err != nil {
			return err
		}
		log.Info(vault.GetHeight(), hex.EncodeToString(hash))
		mc.VaultHashMap[hex.EncodeToString(hash)] = vault
		hashes, ok := mc.VaultHeightMap[vault.GetHeight()]
		if ok {
			mc.VaultHeightMap[vault.GetHeight()] = append(hashes, hex.EncodeToString(hash))
		} else {
			mc.VaultHeightMap[vault.GetHeight()] = []string{hex.EncodeToString(hash)}
		}
	}

	return nil
}

func (mc *MemChain) GetBlockByHash(hash []byte) (*ngtypes.Block, error) {
	mc.RLock()
	defer mc.RUnlock()
	if item, ok := mc.BlockHashMap[hex.EncodeToString(hash)]; !ok {
		return nil, fmt.Errorf("cannot find block:%x in mem", hash)
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
		return nil, fmt.Errorf("cannot find block@%d in mem", height)
	}

	for i := 0; i < len(hashes); i++ {
		if item, ok := mc.BlockHashMap[hashes[i]]; ok {
			return item, nil
		}
	}

	return nil, fmt.Errorf("cannot find block@%d in mem", height)
}

func (mc *MemChain) GetVaultByHash(hash []byte) (*ngtypes.Vault, error) {
	mc.RLock()
	defer mc.RUnlock()

	if item, ok := mc.VaultHashMap[hex.EncodeToString(hash)]; !ok {
		return nil, fmt.Errorf("cannot find vault: %x", hash)
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

	for {
		item, ok := mc.BlockHashMap[hex.EncodeToString(cur.GetPrevHash())]
		if !ok {
			break
		}
		items = append(items, item)
		if cur.IsHead() {
			if v, ok := mc.VaultHashMap[hex.EncodeToString(cur.Header.PrevVaultHash)]; ok {
				items = append(items, v)
			}
		}

		cur = item
	}

	// reverse the slice
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}

	if len(items) > 0 {
		log.Infof("exported chain from %d to %d", items[0].GetHeight(), items[len(items)-1].GetHeight())
	}

	return items
}
