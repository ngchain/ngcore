// Package keytools is the module to reuse the key pair
package keytools

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"

	"github.com/mr-tron/base58"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// ReadLocalKey will read the local AES-256-GCM encrypted secp256k1 key file to load an ecdsa private key.
func ReadLocalKey(filename string, password string) *secp256k1.PrivateKey {
	var key *secp256k1.PrivateKey

	if filename == "" {
		path := GetDefaultFolder()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			err := os.Mkdir(path, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
		filename = GetDefaultFile()
	}

	if _, err := os.Stat(filename); err != nil {
		key = CreateLocalKey(filename, password)
	} else {
		var raw []byte

		raw, err = ioutil.ReadFile(filepath.Clean(filename))
		if err != nil {
			panic(err)
		}

		rawPK := utils.AES256GCMDecrypt(raw, []byte(password))
		key = secp256k1.NewPrivateKey(new(big.Int).SetBytes(rawPK))
	}

	return key
}

// NewLocalKey will create a privateKey only
func NewLocalKey() *secp256k1.PrivateKey {
	key, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		panic(err)
	}

	return key
}

// CreateLocalKey will create a keyfile named *filename* and encrypted with *password* in aes-256-gcm.
func CreateLocalKey(filename, password string) *secp256k1.PrivateKey {
	key := NewLocalKey()

	if filename == "" {
		path := GetDefaultFolder()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			err := os.Mkdir(path, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
		filename = GetDefaultFile()
	}

	// save key to ngcore.key file
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}

	encrypted := utils.AES256GCMEncrypt(key.D.Bytes(), []byte(password))

	_, err = file.Write(encrypted)
	if err != nil {
		panic(err)
	}

	_ = file.Close()

	return key
}

// RecoverLocalKey will recover a keyfile named *filename* with the password from the privateKey string.
func RecoverLocalKey(filename, password, privateKey string) *secp256k1.PrivateKey {
	bKey, err := base58.FastBase58Decoding(privateKey)
	if err != nil {
		panic(err)
	}

	key := secp256k1.NewPrivateKey(new(big.Int).SetBytes(bKey))

	if filename == "" {
		path := GetDefaultFolder()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			err := os.Mkdir(path, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
		filename = GetDefaultFile()
	}

	// save key to ngcore.key file
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}

	encrypted := utils.AES256GCMEncrypt(key.D.Bytes(), []byte(password))

	_, err = file.Write(encrypted)
	if err != nil {
		panic(err)
	}

	_ = file.Close()

	return key
}

// PrintKeysAndAddress will print the **privateKey and its publicKey** to the console.
func PrintKeysAndAddress(privateKey *secp256k1.PrivateKey) {
	rawPrivateKey := privateKey.Serialize() // its D
	fmt.Println("Private Key: ", base58.FastBase58Encoding(rawPrivateKey))

	bPubKey := utils.PublicKey2Bytes(*privateKey.PubKey())
	fmt.Println("Public Key: ", base58.FastBase58Encoding(bPubKey))

	address := ngtypes.NewAddress(privateKey)
	fmt.Println("Address: ", base58.FastBase58Encoding(address))
}
