package keyManager

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"github.com/mr-tron/base58"
	"github.com/ngin-network/ngcore/utils"
	"github.com/whyrusleeping/go-logging"
	"io/ioutil"
	"os"
	"time"
)

var log = logging.MustGetLogger("manager")

// local(self) key, balance, accounts management
// manage all belongs to the localUser (yourself)

type KeyManager struct {
	CurrentKey    *ecdsa.PrivateKey
	passwordCache string
	ngKeyName     string

	LastSyncedTime time.Time
}

// ReadLocal will
func (m *KeyManager) ReadLocalKey() *ecdsa.PrivateKey {
	var key *ecdsa.PrivateKey

	_, err := os.Stat(m.ngKeyName)
	if err != nil {
		key = m.CreateLocalKey()
	} else {
		b, err := ioutil.ReadFile(m.ngKeyName)
		if err != nil {
			log.Panic(err)
		}
		data := utils.DataDecrypt(b, []byte(m.passwordCache))
		key, err = x509.ParseECPrivateKey(data)
		if err != nil {
			log.Panic(err)
		}
	}
	m.CurrentKey = key
	return key
}

func (m *KeyManager) CreateLocalKey() *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	m.CurrentKey = key
	if err != nil {
		log.Panic(err)
	}

	// save key to ngcore.key file
	file, err := os.Create(m.ngKeyName)
	if err != nil {
		log.Panic(err)
	}
	keyData, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		log.Panic(err)
	}

	password := m.passwordCache
	encrypted := utils.DataEncrypt(keyData, []byte(password))
	_, err = file.Write(encrypted)
	if err != nil {
		log.Panic(err)
	}

	file.Close()

	return key
}

func (m *KeyManager) PrintKeyPair() {
	bPrivKey, err := x509.MarshalECPrivateKey(m.CurrentKey)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("Private Key: ", base58.FastBase58Encoding(bPrivKey[:]))
	bPubKey := elliptic.Marshal(elliptic.P256(), m.CurrentKey.PublicKey.X, m.CurrentKey.PublicKey.Y)
	fmt.Println("Public Key: ", base58.FastBase58Encoding(bPubKey[:]))
}

func NewKeyManager(filename string, keyPass string) *KeyManager {
	var ngKeyName string
	if filename == "" {
		ngKeyName = "ngcore.key"
	} else {
		ngKeyName = filename
	}

	return &KeyManager{
		ngKeyName:     ngKeyName,
		passwordCache: keyPass,
	}
}
