// Package ngtypes exposes the chain's base types.
//
// The chain knows two nouns only:
//
//   - Address: the 32-byte keccak hash of a public key — the identity,
//     the balance holder and the namespace. Addresses spend directly;
//     nothing is registered.
//   - Contract: the code slot an address may open under its own
//     namespace (CommitTx against the empty base burns DeployFee).
//     Its Source is plain wat text, changed by committing diff hunks,
//     frozen and executed while active.
//
// Tx verbs: Generate (mining reward), Transact (pay addresses and
// trigger their active contracts), Commit (change your contract
// source), Activate / Deactivate (turn the vm on / off), Destroy
// (remove your slot).
package ngtypes
