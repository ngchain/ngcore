/*
Package ngtypes implements the block structure and related types.

# NGTYPE

## Account

Account is not an unlimited resource, the account is created when register tx and removed with logout tx

## Block

Block is not only a tx mass, but a vault for network status.

It means that the security can be ensured with a specific length sub-chain,
the blocks before the chain can be throw.

The block has several steps to be mature

	(sheetHash)--> BareBlock --(+txs&mtree)--> Unsealing --(+nonce)--> SealedBlock

The sheetHash is the sheet before applying block txs.

## Tx

Tx is a basic operation method in NGIN network, acting as extendable structure for the network's functions.

Tx can handle more than one transfer of coins or operations because of it's values field and participants
field. Also, with the schnorr signature,
the coin can be owned by multi-person at the same time and the account is able to send tx only when all owners signed
the tx.

Currently, there are 5 types of tx

1. Generate Tx: generate tx works when generating the coin, only miner can send this,
and it can also have multi-participants.

2. Register Tx: register the account

3. Logout Tx: logout the account

4. Transaction: normal tx, can be used for sending money, or trigger the vm's onTx function

5. Assign Tx: assign raw bytes to Contract, overwriting

5. Append Tx: append raw bytes to Contract

## Sheet

Sheet is the aggregation of all status, not only accounts, but anonymous addresses

*/
package ngtypes
