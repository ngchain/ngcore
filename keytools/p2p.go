package keytools

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"io/ioutil"
	"os"
	"path/filepath"
)

func readKeyFromFile(filename string) crypto.PrivKey {
	keyFile, err := os.Open(filepath.Clean(filename))
	if err != nil {
		panic(err)
	}

	raw, err := ioutil.ReadAll(keyFile)
	if err != nil {
		panic(err)
	}

	_ = keyFile.Close()

	priv, err := crypto.UnmarshalPrivateKey(raw)
	if err != nil {
		panic(err)
	}

	return priv
}

func GetP2PKey(path string) crypto.PrivKey {
	// read from db / file
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}

		path = filepath.Join(home, ".ngkeys")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			err := os.Mkdir(path, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}

		path = filepath.Join(path, "ngp2p.key")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		priv, _, err := crypto.GenerateKeyPair(crypto.Secp256k1, 256)
		if err != nil {
			panic(err)
		}

		raw, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			panic(err)
		}

		//log.Info("creating bootstrap key")

		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}

		_, _ = f.Write(raw)
		_ = f.Close()
	}

	return readKeyFromFile(path)
}
