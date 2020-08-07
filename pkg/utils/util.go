package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func RandomToken(size int) (string, error) {
	token := make([]byte, size, size)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}
