package keytools

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"github.com/mr-tron/base58"
	"github.com/ngchain/ngcore/utils"
	"github.com/whyrusleeping/go-logging"
	"io/ioutil"
	"os"
)

var log = logging.MustGetLogger("key")

// ReadLocal will
func ReadLocalKey(filename string, password string) *ecdsa.PrivateKey {
	var key *ecdsa.PrivateKey

	if _, err := os.Stat(filename); err != nil {
		key = CreateLocalKey(filename, password)
	} else {
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Panic(err)
		}
		data := utils.DataDecrypt(b, []byte(password))
		key, err = x509.ParseECPrivateKey(data)
		if err != nil {
			log.Panic(err)
		}
	}

	return key
}

func PrintPublicKey(key *ecdsa.PrivateKey) {
	publicKey := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)
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
	bPrivKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("Private Key: ", base58.FastBase58Encoding(bPrivKey))
	bPubKey := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)
	fmt.Println("Public Key: ", base58.FastBase58Encoding(bPubKey))
}
