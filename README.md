# NgCore

New Golang implement of Ngin Network Node Daemon

## NGIN

NGIN is a totally new chain which is not a fork of ethereum or other chain. It is radically updating.

## requirements

go version >= 1.11

## Features 

### Fast Sync

Just Need to sync only 3 *Vaulty* and 30 Blocks before running.

### Low Storage Usage

3 *Vault* and 30 Blocks take few megabytes.

### Account in Numbers

easy to remember for humankind.

### Rapid Transaction

with formatted network grid, the message (operations and broadcasts) between nodes will cost the shortest time.

### Multi-platform Friendly

Golang + WebAssembly + Low Storage Usage

### More ...

## FAQ:

- Q: How to gain an account?


    There are two methods to get one:
    
    1. Continuously mining. when you mined an vault, you will get random new one
    
    2. Buy one from others

- Q: Diff between Block, Vault, Tx ... 's hash? 


    All hashes are SHA3 hash of the protobuf bytes, including blocks'

 
- 

## Temp Notice
```
Structure:
      
        RPC -+          +--> Consensus -> Vault -> Blocks
             +--> Mgr --+ 
        P2p -+          +--> State -+--> Pool -> Txs
                                    +--> Accounts -> Accounts



```
