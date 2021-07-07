package main

const (
	// app.
	usage       = "Brand-new golang daemon implement of ngchain network node"
	description = "The ngchain is a radically updating brand-new blockchain network, " +
		"which is not a fork of ethereum or any other chain."

	// flag values.
	defaultTCPP2PPort = 52520
	defaultRPCHost    = "127.0.0.1"
	defaultRPCPort    = 52521

	defaultDBFolder = "ngdb"
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
