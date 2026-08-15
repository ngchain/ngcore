package ngstate

import (
	"github.com/c0mm4nd/wasman"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// crypto host-op pricing: hashing is cheap per byte, signature
// verification is priced near a coin transfer
const (
	gasKeccakBase    = 100
	gasKeccakPerByte = 1
	gasSigVerify     = 3000
)

// initCryptoImports binds the crypto module, giving contracts the
// chain's own primitives: keccak-256 and per-scheme signature
// verification. This is what contract-level multisig builds on — a
// proposal hashed on chain, off-chain-collected signatures verified
// one by one inside the contract
func initCryptoImports(vm *VM) error {
	// keccak256(ptr, len, out) writes the 32-byte digest to out
	err := vm.linker.DefineAdvancedFunc("crypto", "keccak256", func(ins *wasman.Instance) interface{} {
		return func(ptr, size, out uint32) uint32 {
			vm.charge(gasKeccakBase + gasKeccakPerByte*uint64(size))

			data, err := readMem(ins, ptr, size)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			l, err := cp(ins, out, utils.KeccakSum256(data))
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

	// verify(scheme, pk_ptr, pk_len, hash_ptr, sig_ptr, sig_len) -> i32
	// checks one signature over a 32-byte digest under the scheme
	err = vm.linker.DefineAdvancedFunc("crypto", "verify", func(ins *wasman.Instance) interface{} {
		return func(scheme, pkPtr, pkLen, hashPtr, sigPtr, sigLen uint32) uint32 {
			vm.charge(gasSigVerify)

			pubKey, err := readMem(ins, pkPtr, pkLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}
			hash, err := readMem(ins, hashPtr, 32)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}
			sig, err := readMem(ins, sigPtr, sigLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			if ngtypes.VerifyHashSig(ngtypes.SigScheme(scheme), pubKey, hash, sig) {
				return 1
			}

			return 0
		}
	})
	if err != nil {
		return err
	}

	// addr_of(scheme, pk_ptr, pk_len, out) derives the 32-byte address
	// of a public key, so a contract can whitelist ADDRESSES yet verify
	// against the revealed keys
	err = vm.linker.DefineAdvancedFunc("crypto", "addr_of", func(ins *wasman.Instance) interface{} {
		return func(scheme, pkPtr, pkLen, out uint32) uint32 {
			vm.charge(gasKeccakBase + gasKeccakPerByte*uint64(pkLen))

			pubKey, err := readMem(ins, pkPtr, pkLen)
			if err != nil {
				vm.logger.Error(err)
				return 0
			}

			addr := ngtypes.AddressOfPubKey(ngtypes.SigScheme(scheme), pubKey)
			l, err := cp(ins, out, addr[:])
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

	return nil
}
