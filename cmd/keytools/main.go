package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/ngchain/ngcore/keytools"
)

// basic usages:
// - get the privateKey / PublicKey of one ngKey file
//

func main() {
	flag.Parse()

	switch flag.Arg(0) {
	case "new":
		newKeyPair(flag.Arg(1), flag.Arg(2))
	case "read":
		parseKeyFile(flag.Arg(1), flag.Arg(2))
	default:
		fmt.Println(`
commands:

keytools new [filename] [password]
new a ngKey and print key pair to the console

keytools parse [filename] [password]
read a existed ngKey file and print its data to the console

on default, the filename is ngKey`)
	}
}

func newKeyPair(filename string, pass string) {
	key := keytools.CreateLocalKey(strings.TrimSpace(filename), strings.TrimSpace(pass))
	keytools.PrintKeyPair(key)
}

func parseKeyFile(filename string, pass string) {
	key := keytools.ReadLocalKey(strings.TrimSpace(filename), strings.TrimSpace(pass))
	keytools.PrintKeyPair(key)
}
