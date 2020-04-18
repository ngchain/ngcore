# How to Start NGIN Mainnet (or forknet)

1. Modify the Version & NetworkID in `./ngtypes/defaults.go`

2. Generate a new R & S for Genesis Generate Tx (with `./cmd/genesis` tool)

3. Run a bootstrap node (without mining)

4. Write the bootstrap node into ngp2p config

5. Run a mining node
