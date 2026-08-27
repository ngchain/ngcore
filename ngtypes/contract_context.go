package ngtypes

import (
	"bytes"
	"encoding/hex"
	"sort"
	"sync"

	"github.com/ngchain/ngcore/utils"
)

// ContractContext is the Context field of the Account, which
// is an on-chain k-v storage.
// Keys/Values are the RLP-encoded form and are always kept sorted by key
// so that the encoding is deterministic across nodes.
type ContractContext struct {
	Keys   []string
	Values [][]byte

	mu     sync.RWMutex
	valMap map[string][]byte
}

// NewContractContext craetes a new empty ContractContext
func NewContractContext() *ContractContext {
	return &ContractContext{
		Keys:   make([]string, 0),
		Values: make([][]byte, 0),
		valMap: make(map[string][]byte),
	}
}

// ensureInit rebuilds the internal map after the context was created by
// a decoder (rlp/json) which fills the exported fields only
func (ctx *ContractContext) ensureInit() {
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
func (ctx *ContractContext) Set(key string, val []byte) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	ctx.ensureInit()
	ctx.valMap[key] = val
	ctx.splitMap()
}

// Del removes the key from the context
func (ctx *ContractContext) Del(key string) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	ctx.ensureInit()
	delete(ctx.valMap, key)
	ctx.splitMap()
}

// splitMap flushes valMap into the exported Keys/Values, sorted by key
// to keep the RLP encoding deterministic
func (ctx *ContractContext) splitMap() {
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
func (ctx *ContractContext) Get(key string) []byte {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	ctx.ensureInit()

	return ctx.valMap[key]
}

// Has reports whether the key is present, distinguishing an absent key from a
// key stored with an empty value (both make Get return nil). The storage-deposit
// accounting needs this: an entry with an empty value still owes the bond on its
// key bytes, whereas an absent key owes nothing.
func (ctx *ContractContext) Has(key string) bool {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	ctx.ensureInit()

	_, ok := ctx.valMap[key]
	return ok
}

// Clone returns a deep copy of the context
func (ctx *ContractContext) Clone() *ContractContext {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	ctx.ensureInit()

	clone := NewContractContext()
	for k, v := range ctx.valMap {
		val := make([]byte, len(v))
		copy(val, v)
		clone.valMap[k] = val
	}
	clone.splitMap()

	return clone
}

// Equals checks whether the other is same with this ContractContext
func (ctx *ContractContext) Equals(other *ContractContext) (bool, error) {
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
func (ctx *ContractContext) MarshalJSON() ([]byte, error) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	ctx.ensureInit()

	json := make(map[string]string, len(ctx.valMap))
	for k, v := range ctx.valMap {
		json[k] = hex.EncodeToString(v)
	}

	return utils.JSON.Marshal(json)
}

// UnmarshalJSON decodes the ContractContext from the map with hex values
func (ctx *ContractContext) UnmarshalJSON(raw []byte) error {
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
