package wasm

import (
	"math/big"
	"reflect"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngstate"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

func (vm *VM) InitBuiltInImports() {
	vm.linker.Define("ng", "getNum", wasmtime.WrapFunc(vm.store, func() int64 {
		return int64(vm.num)
	}).AsExtern())

	vm.linker.Define("ng", "createTx", wasmtime.NewFunc(vm.store,
		wasmtime.NewFuncType(
			[]*wasmtime.ValType{
				wasmtime.NewValType(wasmtime.KindI32),
				wasmtime.NewValType(wasmtime.KindI64),
				wasmtime.NewValType(wasmtime.KindI64),
				wasmtime.NewValType(wasmtime.KindI64),
				wasmtime.NewValType(wasmtime.KindI32),
				wasmtime.NewValType(wasmtime.KindI32),
			},
			[]*wasmtime.ValType{
				wasmtime.NewValType(wasmtime.KindI32),
				wasmtime.NewValType(wasmtime.KindI32),
			},
		),
		func(caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
			// txType int32, to int64, value int64, fee int64, extraPtr, extraLen int32) (rawPtr, rawLen int32)
			var (
				txType   = vals[0].I32()
				to       = vals[1].I64()
				value    = vals[2].I64()
				fee      = vals[3].I64()
				extraPtr = vals[4].I32()
				extraLen = vals[5].I32()
			)

			ty := ngtypes.TxType(txType)
			if ty == ngtypes.TxType_REGISTER {
				return []wasmtime.Val{
					wasmtime.ValI32(0), wasmtime.ValI32(0),
				}, wasmtime.NewTrap(vm.store, "try register in contract")
			}

			var participants = make([][]byte, 1)
			account, err := ngstate.GetAccountByNum(uint64(to))
			if err != nil {
				return []wasmtime.Val{
					wasmtime.ValI32(0), wasmtime.ValI32(0),
				}, wasmtime.NewTrap(vm.store, "try register in contract")
			}
			participants[0] = account.Owner

			var values = make([]*big.Int, 1)

			values[0] = new(big.Int).SetUint64(uint64(value))

			bigFee := new(big.Int).SetUint64(uint64(fee))

			extra := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(extraPtr),
				Len:  int(extraLen),
				Cap:  int(extraLen),
			}))

			tx := ngtypes.NewUnsignedTx(
				ngtypes.TxType_TRANSACTION,
				ngchain.GetLatestBlockHash(),
				vm.num,
				participants,
				values,
				bigFee,
				extra,
			)

			// providing Proto encoded bytes
			// Reason: 1. avoid accident client modification 2. less length
			rawTx, err := utils.Proto.Marshal(tx)
			if err != nil {
				return []wasmtime.Val{
					wasmtime.ValI32(0), wasmtime.ValI32(0),
				}, wasmtime.NewTrap(vm.store, err.Error())
			}

			return []wasmtime.Val{
				wasmtime.ValI32(int32(uintptr(unsafe.Pointer(&rawTx)))), wasmtime.ValI32(int32(len(rawTx))),
			}, nil
		}).AsExtern(),
	)
}
