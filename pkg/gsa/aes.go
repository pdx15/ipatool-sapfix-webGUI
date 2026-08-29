package gsa

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

// cbcDecrypt performs AES-256-CBC decryption with PKCS#7 padding removal, as
// used for the GSA "spd" payload. The key and IV are derived from the SRP
// session key via SPDKeyIV.
func cbcDecrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) == 0 || len(ciphertext)%block.BlockSize() != 0 {
		return nil, errors.New("invalid spd ciphertext length")
	}

	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)

	return pkcs7Unpad(plaintext, block.BlockSize())
}

// pkcs7Unpad strips PKCS#7 padding and verifies its validity. Invalid padding
// (wrong key or corrupted data) returns an error rather than silently
// producing garbage.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("invalid spd plaintext")
	}

	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > blockSize || padLen > len(data) {
		return nil, errors.New("invalid spd PKCS#7 padding")
	}

	for _, b := range data[len(data)-padLen:] {
		if int(b) != padLen {
			return nil, errors.New("invalid spd PKCS#7 padding")
		}
	}

	return data[:len(data)-padLen], nil
}
