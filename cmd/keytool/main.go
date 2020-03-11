package main

import (
	"flag"
	"fmt"
	"github.com/ngin-network/ngcore/keyManager"
	"strings"
)

// basic usages:
// - get the privateKey / PublicKey of one ngKey file
//

func main() {
	flag.Parse()

	switch flag.Arg(0) {
	case "new":
		newKeyPair(flag.Arg(1), flag.Arg(2))
		break
	case "read":
		parseKeyFile(flag.Arg(1), flag.Arg(2))
		break
	default:
		fmt.Println(`
commands:

keyTool new [filename] [password]
new a ngKey and print key pair to the console

keyTool parse [filename] [password]
read a existed ngKey file and print its data to the console

on default, the filename is ngKey`)

	}

}

func newKeyPair(filename string, pass string) {
	localMgr := keyManager.NewKeyManager(strings.TrimSpace(filename), strings.TrimSpace(pass))
	localMgr.CreateLocalKey()
	localMgr.PrintKeyPair()
}

func parseKeyFile(filename string, pass string) {
	localMgr := keyManager.NewKeyManager(strings.TrimSpace(filename), strings.TrimSpace(pass))
	localMgr.ReadLocalKey()
	localMgr.PrintKeyPair()
}
