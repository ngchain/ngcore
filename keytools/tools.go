package keytools

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/mr-tron/base58"
	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/ngcore/utils"
)

var log = logging.MustGetLogger("key")

// ReadLocalKey will read the local x509 key file to load an ecdsa private key
func ReadLocalKey(filename string, password string) *ecdsa.PrivateKey {
	var key *ecdsa.PrivateKey

	if _, err := os.Stat(filename); err != nil {
		key = CreateLocalKey(filename, password)
	} else {
		var raw []byte

		raw, err = ioutil.ReadFile(filename)
		if err != nil {
			log.Panic(err)
		}
		data := utils.DataDecrypt(raw, []byte(password))
		key, err = x509.ParseECPrivateKey(data)
		if err != nil {
			log.Panic(err)
		}
	}

	return key
}

func PrintPublicKey(key *ecdsa.PrivateKey) {
	publicKey := utils.ECDSAPublicKey2Bytes(key.PublicKey)
	log.Warningf("PublicKey is bs58: %v\n", base58.FastBase58Encoding(publicKey))
}

func CreateLocalKey(filename string, password string) *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Panic(err)
	}

	// save key to ngcore.key file
	file, err := os.Create(filename)
	if err != nil {
		log.Panic(err)
	}
	keyData, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		log.Panic(err)
	}

	encrypted := utils.DataEncrypt(keyData, []byte(password))
	_, err = file.Write(encrypted)
	if err != nil {
		log.Panic(err)
	}

	_ = file.Close()

	return key
}

func PrintKeyPair(key *ecdsa.PrivateKey) {
	rawPrivateKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		log.Panic(err)
	}

	fmt.Println("Private Key: ", base58.FastBase58Encoding(rawPrivateKey))
	bPubKey := utils.ECDSAPublicKey2Bytes(key.PublicKey)
	fmt.Println("Public Key: ", base58.FastBase58Encoding(bPubKey))
}
