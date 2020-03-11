/*
Package ngtypes implements the block structure and related types.

# NGTYPE

## Account

Account is not an unlimited resource, the account is born on block generated (if the miner dont have one)

## Block

Block is just a tx mass, but can be treat as a block

It means that after the vault summary, the data in previous block chain can be throw

We just need to keep the latest some block and treasuries to make the chain safe

	bare -(+txs&mtree)-> Unsealing -(+nonce)-> Sealed

## Operation

## Sheet

## Vault

*/

package ngtypes
