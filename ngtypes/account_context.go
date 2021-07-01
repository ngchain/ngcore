package ngtypes

import (
	"bytes"
	"encoding/hex"
	"sync"

	"github.com/ngchain/ngcore/utils"
)

type AccountContext struct {
	Keys   []string
	Values [][]byte

	mu     *sync.RWMutex
	valMap map[string][]byte
}

func NewAccountContext() *AccountContext {
	return &AccountContext{
		Keys:   make([]string, 0),
		Values: make([][]byte, 0),
		valMap: make(map[string][]byte),
	}
}

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

func (ctx *AccountContext) Get(key string) []byte {
	ctx.mu.RLock()
	ret := ctx.valMap[key]
	ctx.mu.RUnlock()
	return ret
}

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

func (ctx *AccountContext) MarshalJSON() ([]byte, error) {
	json := make(map[string]string, len(ctx.valMap))
	for k, v := range ctx.valMap {
		json[k] = hex.EncodeToString(v)
	}

	return utils.JSON.Marshal(json)
}

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
