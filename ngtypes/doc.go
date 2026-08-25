// Package ngtypes exposes the chain's base types.
//
// The chain knows two nouns only:
//
//   - Address: the 32-byte blake3 hash of a public key — the identity,
//     the balance holder and the namespace. Addresses spend directly;
//     nothing is registered.
//   - Contract: the code slot an address may open under its own
//     namespace (a DeployTx against the empty base opens it). Its Source
//     is a compiled wasm module, replaced wholesale by a later DeployTx
//     (UUPS upgrade), and live from the moment it is deployed.
//
// Tx verbs: Generate (mining reward), Transact (pay addresses and
// trigger their live contracts), Deploy (deploy your contract, or upgrade
// it UUPS-style), Destroy (remove your slot).
package ngtypes
