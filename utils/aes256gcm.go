package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	"golang.org/x/crypto/sha3"
)

func AES256GCMEncrypt(raw []byte, password []byte) (encrypted []byte) {
	hashPassword := sha3.Sum256(password)
	c, err := aes.NewCipher(hashPassword[:])
	if err != nil {
		panic(err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		panic(err)
	}

	nonce := make([]byte, gcm.NonceSize())
	_, _ = rand.Read(nonce)

	return gcm.Seal(nonce, nonce, raw, nil)
}

func AES256GCMDecrypt(raw []byte, password []byte) (decrypted []byte) {
	hashPassword := sha3.Sum256(password)
	c, err := aes.NewCipher(hashPassword[:])
	if err != nil {
		panic(err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		panic(err)
	}

	nonce, encrypted := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	decrypted, err = gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		panic(err)
	}

	return decrypted
}
