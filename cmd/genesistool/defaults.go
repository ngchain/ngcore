package main

const (
	name        = "genesistool"
	usage       = "Helper for generating initial variables for genesis items in ngchain"
	description = `This tool is set for chain developers to check the correctness of the genesis information (e.g. tx, block ...). 
And it will give suggestion values to help correct the chain's genesis info'`
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
