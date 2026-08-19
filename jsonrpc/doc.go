// Package jsonrpc is the json-rpc2 server of ngcore.
//
// Encoding conventions, identical on every human-facing surface (see
// docs/rpc.md for the method reference):
//   - addresses (and keys) are bs58 strings
//   - all other raw bytes (hashes, code, calldata, RLP payloads) are
//     lowercase hex, never base64
//   - the float64 `value`/`fee` params of the tx-composition methods are
//     in whole NG (18-decimal raw units on chain); balances return as
//     decimal strings of raw units
package jsonrpc
