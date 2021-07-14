package main

const (
	name        = "coreminer"
	usage       = "Miner for ngchain PoW protocol"
	description = `This is the mining software for solo/local mining, 
the mining share from which will all get sent to the local ngcore.
For pool mining, please download from https://github.com/ngchain/ngminer
`

	defaultRPCHost = "127.0.0.1"
	defaultRPCPort = 52521
)

var (
	Commit  string // from `git rev-parse HEAD`
	Tag     string // from `git describe --tags --abbrev=0`
	Version string
)

func init() {
	if Tag == "" && Commit == "" {
		panic("invalid version: tag:" + Tag + " commit: " + Commit)
	}
	if Tag != "" {
		Version = Tag
	} else {
		Version = "v0.0.0-" + Commit
	}
}
