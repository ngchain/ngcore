package main

const (
	// app.
	name        = "ngcore"
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
	switch {
	case Tag != "":
		Version = Tag
	case Commit != "":
		Version = "v0.0.0-" + Commit
	default:
		Version = "v0.0.0-dev" // a plain go build, no ldflags injected
	}
}
