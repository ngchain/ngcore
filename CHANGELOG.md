# ChangeLog

## v0.0.17

- BUGFIX: fix difficulty algorithm
- DONE: add height bomb for difficulty algorithm
- DONE: add keytools functions
- DONE: rename gen to gentools
- DONE: change default key and db path
- DONE: save log into log file
- DONE: use fmt to output
- DONE: update genesis block time
- DONE: upgrade P2P version

## v0.0.16

- BUGFIX: fix jsonrpc
- DONE: add kad-dht and mdns for peer discovery
- REMOVE: temporarily remove bazel
- DONE: use github action CI instead of circleCI
- DONE: add auto-fork mechanism
- BUGFIX: fix miner's job update on receiving p2p broadcasts
- BUGFIX: fix some deadlocks
- DONE: update genesis block time
- DONE: upgrade P2P version
- DONE: test new difficulty algorithm

## v0.0.15

- DONE: optimize built-in miner
- DONE: avoid mem leak
- DONE: fix tx check
- DONE: pass ngwallet basic test

## v0.0.14

- DONE: change PoW algorithm from cryptonight-go to RandomNG
- DONE: add submitWork and getWork
- DONE: update genesis block
- BUGFIX: nonce length => 8

## v0.0.13

- DONE: huge changes on JSON RPC
- DONE: test and fix RegisterTx
- DONE: fix some bugs on tx
- DONE: remove useless height
- DONE: add prevBlockHash for identification
- DONE: same changes to state
- DONE: now we can use prevBlockHash to verify whether the tx is on the correct height in TxPool.PutTx
- DONE: fix checkRegister by adding newAccountNum check
- DONE: recv and bcast Tx
- DONE: fix wrong regTx extra len requirement
- DONE: api params
- DONE: apply tx into state
- DONE: speed up sync
- DONE: take a tx test on ngwallet

## v0.0.12

- DONE: add jsonrpc { GBT, submitBlock, getNetwork, getPeers, getLatest }
- DONE: upgrade deps
- DONE: optimize codes

## v0.0.11

- DONE: Introducing Address to avoid potential public key collision 
- DONE: Finish new ngstate
- TODO: Unit tests for ngstate

## v0.0.10

- Initialized and getting ready for v0.0.11
