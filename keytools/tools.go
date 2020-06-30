// Package keytools is the module to reuse the key pair
package keytools

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"

	logging "github.com/ipfs/go-log/v2"
	"github.com/mr-tron/base58"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("key")

// ReadLocalKey will read the local AES-256-GCM encrypted secp256k1 key file to load an ecdsa private key.
func ReadLocalKey(filename string, password string) *secp256k1.PrivateKey {
	var key *secp256k1.PrivateKey

	if _, err := os.Stat(filename); err != nil {
		key = CreateLocalKey(filename, password)
	} else {
		var raw []byte

		raw, err = ioutil.ReadFile(filepath.Clean(filename))
		if err != nil {
			log.Panic(err)
		}

		rawPK := utils.AES256GCMDecrypt(raw, []byte(password))
		key = secp256k1.NewPrivateKey(new(big.Int).SetBytes(rawPK))
	}

	return key
}

// CreateLocalKey will create a keyfile named *filename* and encrypted with *password* in aes-256-gcm.
func CreateLocalKey(filename string, password string) *secp256k1.PrivateKey {
	key, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		log.Panic(err)
	}

	// save key to ngcore.key file
	file, err := os.Create(filename)
	if err != nil {
		log.Panic(err)
	}

	encrypted := utils.AES256GCMEncrypt(key.D.Bytes(), []byte(password))

	_, err = file.Write(encrypted)
	if err != nil {
		log.Panic(err)
	}

	_ = file.Close()

	return key
}

// PrintAddress will print the privateKey's **address** to the console.
func PrintAddress(privateKey *secp256k1.PrivateKey) {
	address := ngtypes.NewAddress(privateKey)
	log.Warnf("Address is bs58: %s\n", base58.FastBase58Encoding(address))
}

// PrintKeyPair will print the **privateKey and its publicKey** to the console.
func PrintKeyPair(privateKey *secp256k1.PrivateKey) {
	rawPrivateKey := privateKey.Serialize() // its D
	fmt.Println("Private Key: ", base58.FastBase58Encoding(rawPrivateKey))

	bPubKey := utils.PublicKey2Bytes(*privateKey.PubKey())
	fmt.Println("Public Key: ", base58.FastBase58Encoding(bPubKey))

	address := ngtypes.NewAddress(privateKey)
	fmt.Println("Address: ", base58.FastBase58Encoding(address))
}
