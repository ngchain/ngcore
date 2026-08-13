package ngtypes

import (
	"bytes"
	"encoding/hex"
	"sort"
	"sync"

	"github.com/ngchain/ngcore/utils"
)

// AccountContext is the Context field of the Account, which
// is an on-chain k-v storage.
// Keys/Values are the RLP-encoded form and are always kept sorted by key
// so that the encoding is deterministic across nodes.
type AccountContext struct {
	Keys   []string
	Values [][]byte

	mu     sync.RWMutex
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

// ensureInit rebuilds the internal map after the context was created by
// a decoder (rlp/json) which fills the exported fields only
func (ctx *AccountContext) ensureInit() {
	if ctx.valMap == nil {
		ctx.valMap = make(map[string][]byte, len(ctx.Keys))
		for i := range ctx.Keys {
			if i < len(ctx.Values) {
				ctx.valMap[ctx.Keys[i]] = ctx.Values[i]
			}
		}
	}
}

// Set the k-v data
func (ctx *AccountContext) Set(key string, val []byte) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	ctx.ensureInit()
	ctx.valMap[key] = val
	ctx.splitMap()
}

// Del removes the key from the context
func (ctx *AccountContext) Del(key string) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	ctx.ensureInit()
	delete(ctx.valMap, key)
	ctx.splitMap()
}

// splitMap flushes valMap into the exported Keys/Values, sorted by key
// to keep the RLP encoding deterministic
func (ctx *AccountContext) splitMap() {
	keys := make([]string, 0, len(ctx.valMap))
	for k := range ctx.valMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	values := make([][]byte, len(keys))
	for i, k := range keys {
		values[i] = ctx.valMap[k]
	}

	ctx.Keys = keys
	ctx.Values = values
}

// Get the value by key
func (ctx *AccountContext) Get(key string) []byte {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	ctx.ensureInit()

	return ctx.valMap[key]
}

// Clone returns a deep copy of the context
func (ctx *AccountContext) Clone() *AccountContext {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	ctx.ensureInit()

	clone := NewAccountContext()
	for k, v := range ctx.valMap {
		val := make([]byte, len(v))
		copy(val, v)
		clone.valMap[k] = val
	}
	clone.splitMap()

	return clone
}

// Equals checks whether the other is same with this AccountContext
func (ctx *AccountContext) Equals(other *AccountContext) (bool, error) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	ctx.ensureInit()

	other.mu.RLock()
	defer other.mu.RUnlock()
	other.ensureInit()

	if len(ctx.valMap) != len(other.valMap) {
		return false, nil
	}

	for k, v := range ctx.valMap {
		otherV, ok := other.valMap[k]
		if !ok || !bytes.Equal(v, otherV) {
			return false, nil
		}
	}

	return true, nil
}

// MarshalJSON encodes the context as a map, with hex-encoded values
func (ctx *AccountContext) MarshalJSON() ([]byte, error) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	ctx.ensureInit()

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

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	ctx.valMap = valMap
	ctx.splitMap()

	return nil
}
