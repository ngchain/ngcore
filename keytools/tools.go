// Package keytools is the module to reuse the key pair
package keytools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mr-tron/base58"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// ReadLocalKey will read the local AES-256-GCM encrypted key file to
// load a private key (scheme byte + 32-byte seed).
func ReadLocalKey(filename string, password string) *ngtypes.PrivateKey {
	var key *ngtypes.PrivateKey

	if filename == "" {
		path := GetDefaultFolder()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			err := os.Mkdir(path, 0o700)
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

		raw, err = os.ReadFile(filepath.Clean(filename))
		if err != nil {
			panic(err)
		}

		rawPK := utils.AES256GCMDecrypt(raw, []byte(password))
		key, err = ngtypes.ParsePrivateKey(rawPK)
		if err != nil {
			panic(err)
		}
	}

	return key
}

// NewLocalKey will create a privateKey only.
func NewLocalKey() *ngtypes.PrivateKey {
	key, err := ngtypes.GenerateKey()
	if err != nil {
		panic(err)
	}

	return key
}

// CreateLocalKey will create a keyfile named *filename* and encrypted with *password* in aes-256-gcm.
func CreateLocalKey(filename, password string) *ngtypes.PrivateKey {
	return CreateLocalKeyWithScheme(filename, password, ngtypes.SchemeDefault)
}

// CreateLocalKeyWithScheme creates a keyfile under the chosen
// signature scheme (see ngtypes.SigScheme)
func CreateLocalKeyWithScheme(filename, password string, scheme ngtypes.SigScheme) *ngtypes.PrivateKey {
	key, err := ngtypes.GenerateSchemeKey(scheme)
	if err != nil {
		panic(err)
	}

	if filename == "" {
		path := GetDefaultFolder()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			err := os.Mkdir(path, 0o700)
			if err != nil {
				panic(err)
			}
		}

		filename = GetDefaultFile()
	}

	// save key to ngcore.key file
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		panic(err)
	}

	encrypted := utils.AES256GCMEncrypt(key.Serialize(), []byte(password))

	_, err = file.Write(encrypted)
	if err != nil {
		panic(err)
	}

	_ = file.Close()

	return key
}

// RecoverLocalKey will recover a keyfile named *filename* with the password from the privateKey string.
func RecoverLocalKey(filename, password, privateKey string) *ngtypes.PrivateKey {
	bKey, err := base58.FastBase58Decoding(privateKey)
	if err != nil {
		panic(err)
	}

	key, err := ngtypes.ParsePrivateKey(bKey)
	if err != nil {
		panic(err)
	}

	if filename == "" {
		path := GetDefaultFolder()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			err := os.Mkdir(path, 0o700)
			if err != nil {
				panic(err)
			}
		}

		filename = GetDefaultFile()
	}

	// save key to ngcore.key file
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		panic(err)
	}

	encrypted := utils.AES256GCMEncrypt(key.Serialize(), []byte(password))

	_, err = file.Write(encrypted)
	if err != nil {
		panic(err)
	}

	_ = file.Close()

	return key
}

// PrintKeysAndAddress will print the **privateKey and its publicKey** to the console.
func PrintKeysAndAddress(privateKey *ngtypes.PrivateKey) {
	fmt.Println("private key: ", base58.FastBase58Encoding(privateKey.Serialize()))
	fmt.Println("public key: ", base58.FastBase58Encoding(privateKey.PublicBytes()))

	address := ngtypes.NewAddress(privateKey)
	fmt.Println("address: ", address.BS58())
}
