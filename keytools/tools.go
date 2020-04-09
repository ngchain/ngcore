package keytools

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"os"

	"github.com/mr-tron/base58"
	"github.com/whyrusleeping/go-logging"

	"github.com/ngchain/secp256k1"

	"github.com/ngchain/ngcore/utils"
)

var log = logging.MustGetLogger("key")

// ReadLocalKey will read the local x509 key file to load an ecdsa private key
func ReadLocalKey(filename string, password string) *secp256k1.PrivateKey {
	var key *secp256k1.PrivateKey

	if _, err := os.Stat(filename); err != nil {
		key = CreateLocalKey(filename, password)
	} else {
		var raw []byte

		raw, err = ioutil.ReadFile(filename)
		if err != nil {
			log.Panic(err)
		}
		rawPK := utils.AES256GCMDecrypt(raw, []byte(password))
		key = secp256k1.NewPrivateKey(new(big.Int).SetBytes(rawPK))
	}

	return key
}

func PrintPublicKey(key *secp256k1.PrivateKey) {
	publicKey := utils.PublicKey2Bytes(*key.PubKey())
	log.Warningf("PublicKey is bs58: %v\n", base58.FastBase58Encoding(publicKey))
}

func CreateLocalKey(filename string, password string) *secp256k1.PrivateKey {
	key, err := secp256k1.GeneratePrivateKey()
	// key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

func PrintKeyPair(key *secp256k1.PrivateKey) {
	rawPrivateKey := key.D.Bytes()

	fmt.Println("Private Key: ", base58.FastBase58Encoding(rawPrivateKey))
	bPubKey := utils.PublicKey2Bytes(*key.PubKey())
	fmt.Println("Public Key: ", base58.FastBase58Encoding(bPubKey))
}
