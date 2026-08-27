package ngtypes

import "math/big"

// Storage-deposit (state-rent) parameters, gated by ForkStateRent.
//
// The model is a refundable DEPOSIT (a bond), NOT recurring rent: the deposit a
// contract owes is a pure function of the bytes it currently stores in its
// on-chain kv,
//
//	deposit(entry) = DepositPerByte * (len(key) + len(value))
//
// summed over its non-reserved entries. Each kv write moves only the DELTA of
// that function into or out of the protocol escrow (StorageDepositEscrow); a
// destroy refunds the whole sum. No running per-contract total is ever
// persisted — the deposit is always recomputed from the bytes on chain — so the
// rule cannot drift out of sync with storage.

// DepositPerByte is the bond LOCKED per stored byte (key+value) of contract kv.
// At 1e12 pico-NG it is 0.000001 NG/byte (NG has 18 decimals): a meaningful but
// small bond — a 1 KiB entry locks ~0.001 NG — enough to price storage and to
// make deletion worthwhile, while staying negligible against a normal balance.
// It is refundable in full, so it is a deposit, not a fee.
var DepositPerByte = big.NewInt(1_000_000_000_000) // 1e12 pico-NG = 1e-6 NG per byte

// StorageDepositEscrowBase58 is the reserved, unspendable escrow address that
// holds every locked storage deposit. Like GenesisAddress it is a fixed 32-byte
// address no public key hashes to (so nobody can move its funds), but it is a
// DISTINCT constant: all-0x01 rather than all-zero. The escrow starts at zero
// balance and only ever holds the sum of the locked deposits; lock/refund are
// balanced, so supply is conserved.
const StorageDepositEscrowBase58 = "4vJ9JU1bJJE96FWSJKvHsmmFADCg4gpZQff4P3bkLKi"

// StorageDepositEscrow is the decoded escrow address (32 bytes, all 0x01).
var StorageDepositEscrow = storageDepositEscrowAddr()

func storageDepositEscrowAddr() Address {
	var a Address
	for i := range a {
		a[i] = 0x01
	}
	return a
}
