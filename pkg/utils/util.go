package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func RandomToken(size int) (string, error) {
	token := make([]byte, size)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

func UniqueArray(origin []string) (unique []string) {
	unique = make([]string, 0)
	for i := 0; i < len(origin); i++ {
		repeat := false
		for j := i + 1; j < len(origin); j++ {
			if origin[i] == origin[j] {
				repeat = true
				break
			}
		}
		if !repeat {
			unique = append(unique, origin[i])
		}
	}
	return
}
