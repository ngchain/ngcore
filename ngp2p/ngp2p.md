# NgP2P

## Discovery

Based on ipfs(libp2p) network

## Wired

### Methods

#### Ping

Payload: nil

Target: wait for Pong

#### Pong

Payload: nil

Target: If ping's the version / network_id is suitable, return pong, and then the node shall add the remote one to the
peer storage.

#### Reject

Payload: nil

Target: If ping's the version / network_id is NOT suitable, return reject, and then the node shall remove the remote one from the
peer storage.

#### Broadcast

##### Block

##### BlockWithVault

##### Transaction

payload:  

#### Get

##### Blocks

##### Vaults

##### TxPool
