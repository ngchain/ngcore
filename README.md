# NgDaemon
New Golang implement of Ngin Network Node Daemon

## requirements

go version >= 1.12

## Features 

### Fast Sync

Just Need to sync only 2 *treasury* and 12 Blocks.

### Low Storage Usage

2 *treasury* and 12 Blocks take few mega bytes.

### Number Account

easy to remember for humankind.

### Rapid Operation

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
