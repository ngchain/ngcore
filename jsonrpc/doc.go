// Package jsonrpc is the json-rpc2 server of ngcore.
//
// Encoding conventions, identical on every human-facing surface (see
// docs/rpc.md for the method reference):
//   - addresses (and keys) are bs58 strings
//   - all other raw bytes (hashes, code, calldata, RLP payloads) are
//     lowercase hex, never base64
//   - money never touches a float: tx-composition `value`/`fee` params
//     are DECIMAL STRINGS of whole NG ("1.5", parsed exactly); balances
//     return as decimal strings of raw units (NG is 18-decimal)
package jsonrpc
