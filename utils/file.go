package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
	"os"
)

// 加密字符串
func DataEncrypt(b []byte, password []byte) []byte {
	encrypted := make([]byte, len(b))

	dk := sha256.Sum256(password)
	password = dk[:]

	c, err := aes.NewCipher(password)
	if err != nil {
		fmt.Printf("Error: NewCipher(%d bytes) = %s", len(password), err)
		os.Exit(-1)
	}
	size := c.BlockSize()

	cipher.NewCFBEncrypter(c, password[:size]).XORKeyStream(encrypted, b)

	return encrypted
}

// 解密字符串
func DataDecrypt(b []byte, password []byte) []byte {
	decrypted := make([]byte, len(b))

	dk := sha256.Sum256(password)
	password = dk[:]

	c, err := aes.NewCipher(password)
	if err != nil {
		fmt.Printf("Error: NewCipher(%d bytes) = %s", len(password), err)
		os.Exit(-1)
	}
	size := c.BlockSize()
	cipher.NewCFBDecrypter(c, password[:size]).XORKeyStream(decrypted, b)

	return decrypted
}
