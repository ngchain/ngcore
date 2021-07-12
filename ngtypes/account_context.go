package ngtypes

import (
	"bytes"
	"encoding/hex"
	"sync"

	"github.com/ngchain/ngcore/utils"
)

// AccountContext is the Context field of the Account, which
// is a in-mem (on-chain) k-v storage
type AccountContext struct {
	Keys   []string
	Values [][]byte

	mu     *sync.RWMutex
	valMap map[string][]byte
}

// NewAccountContext craetes a new empty AccountContext
func NewAccountContext() *AccountContext {
	return &AccountContext{
		Keys:   make([]string, 0),
		Values: make([][]byte, 0),
		valMap: make(map[string][]byte),
	}
}

// Set the k-v data
func (ctx *AccountContext) Set(key string, val []byte) {
	ctx.mu.Lock()

	ctx.valMap[key] = val
	ctx.splitMap()

	ctx.mu.Unlock()
}

func (ctx *AccountContext) splitMap() {
	itemNum := len(ctx.valMap)

	keys := make([]string, itemNum)
	values := make([][]byte, itemNum)
	i := 0
	for k, v := range ctx.valMap {
		keys[i] = k
		values[i] = v
		i++
	}

	ctx.Keys = keys
	ctx.Values = values
}

// Get the value by key
func (ctx *AccountContext) Get(key string) []byte {
	ctx.mu.RLock()
	ret := ctx.valMap[key]
	ctx.mu.RUnlock()
	return ret
}

// Equals checks whether the other is same with this AccountContext
func (ctx *AccountContext) Equals(other *AccountContext) (bool, error) {
	if len(ctx.valMap) != len(other.valMap) {
		return false, nil
	}

	for i := range other.valMap {
		if !bytes.Equal(other.valMap[i], ctx.valMap[i]) {
			return false, nil
		}
	}

	return true, nil
}

// MarshalJSON encodes the context as a map, with hex-encoded values
func (ctx *AccountContext) MarshalJSON() ([]byte, error) {
	json := make(map[string]string, len(ctx.valMap))
	for k, v := range ctx.valMap {
		json[k] = hex.EncodeToString(v)
	}

	return utils.JSON.Marshal(json)
}

// UnmarshalJSON decodes the AccountContext from the map with hex values
func (ctx *AccountContext) UnmarshalJSON(raw []byte) error {
	var json map[string]string
	err := utils.JSON.Unmarshal(raw, &json)
	if err != nil {
		return err
	}

	valMap := make(map[string][]byte)
	for k, v := range json {
		val, err := hex.DecodeString(v)
		if err != nil {
			return err
		}

		valMap[k] = val
	}

	ctx.valMap = valMap
	return nil
}
