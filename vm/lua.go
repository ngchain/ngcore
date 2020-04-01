package vm

import (
	"github.com/whyrusleeping/go-logging"
	lua "github.com/yuin/gopher-lua"
)

var log = logging.MustGetLogger("chain")

type LuaVM struct {
	*lua.LState
}

// NewLuaVM creates a new contract VM based on Lua
func NewLuaVM() *LuaVM {
	L := lua.NewState(lua.Options{
		RegistrySize:     1024 * 20, // this is the initial size of the registry
		RegistryMaxSize:  1024 * 80, // this is the maximum size that the registry can grow to. If set to `0` (the default) then the registry will not auto grow
		RegistryGrowStep: 32,        // this is how much to step up the registry by each time it runs out of space. The default is `32`.

		CallStackSize:       120,  // this is the maximum callstack size of this LState
		MinimizeStackMemory: true, // Defaults to `false` if not specified. If set, the callstack will auto grow and shrink as needed up to a max of `CallStackSize`. If not set, the callstack will be fixed at `CallStackSize`.

		SkipOpenLibs:        false,
		IncludeGoStackTrace: false,
	})
	return &LuaVM{
		LState: L,
	}
}

func (vm LuaVM) RunState(raw []byte) []byte {
	err := vm.DoString(string(raw))
	if err != nil {
		log.Error(err)
		return nil
	}
	return nil
}
