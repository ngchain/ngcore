package ngstate

import (
	"math/big"

	"github.com/c0mm4nd/wasman"
	"github.com/pkg/errors"
)

// Money crosses the host ABI as a FIXED 32-byte little-endian value — the
// same wire format the u256 wideint module and the token standard use, so
// native NG (18 decimals, big.Int on chain) and token amounts share one
// representation with full 256-bit range.

// readBigLE reads a 32-byte little-endian amount out of linear memory
func readBigLE(ins *wasman.Instance, ptr uint32) (*big.Int, error) {
	raw, err := readMem(ins, ptr, 32)
	if err != nil {
		return nil, err
	}
	be := make([]byte, 32)
	for i, b := range raw {
		be[31-i] = b
	}
	return new(big.Int).SetBytes(be), nil
}

// bigToLE32 encodes a non-negative amount as a fixed 32-byte little-endian
// value (the money wire format); values wider than 256 bits are truncated
// to the low 32 bytes, which cannot happen for real balances
func bigToLE32(v *big.Int) []byte {
	be := v.Bytes()
	le := make([]byte, 32)
	for i := 0; i < len(be) && i < 32; i++ {
		le[i] = be[len(be)-1-i]
	}
	return le
}

// writeBigLE writes an amount as 32-byte little-endian into linear memory
func writeBigLE(ins *wasman.Instance, ptr uint32, v *big.Int) (uint32, error) {
	be := v.Bytes()
	if len(be) > 32 {
		return 0, errors.New("amount exceeds 256 bits")
	}
	le := make([]byte, 32)
	for i, b := range be {
		le[len(be)-1-i] = b
	}
	return cp(ins, ptr, le)
}

func initCoinImports(vm *VM) error {
	// get_balance writes the address's native balance as 32-byte LE
	err := vm.linker.DefineAdvancedFunc("coin", "get_balance", func(ins *wasman.Instance) interface{} {
		return func(addrPtr, ptr uint32) uint32 {
			vm.charge(gasKVRead)

			addr, err := readAddr(ins, addrPtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := writeBigLE(ins, ptr, vm.journal.balanceOf(vm.txn, addr))
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			return l
		}
	})
	if err != nil {
		return err
	}

	// transfer moves a 32-byte LE amount from the EXECUTING address to
	// the `to` address, through the journal: nothing is final until the
	// whole call succeeds. Within a service call the callee spends its
	// own funds, never the caller's
	err = vm.linker.DefineAdvancedFunc("coin", "transfer", func(ins *wasman.Instance) interface{} {
		return func(toPtr, valuePtr uint32) uint32 {
			vm.charge(gasCoinTransfer)

			to, err := readAddr(ins, toPtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			value, err := readBigLE(ins, valuePtr)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			from := vm.currentAddress()
			err = vm.journal.transfer(vm.txn, from, to, value)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			// surface the internal transfer as a log (queryable via
			// ng_getLogs) — the emitter is the sender, data is to ‖ value.
			// Same cap as user events so a transfer loop cannot bloat the receipt
			if len(vm.events) < maxEventsPerRun {
				data := make([]byte, 64)
				copy(data[:32], to[:])
				copy(data[32:], bigToLE32(value))
				vm.events = append(vm.events, Event{
					Contract: from.Bytes(),
					Topic:    EventTopicTransfer,
					Data:     data,
				})
			}

			return 1
		}
	})
	if err != nil {
		return err
	}

	return nil
}
