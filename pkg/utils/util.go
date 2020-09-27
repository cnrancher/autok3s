package utils

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
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

func AskForConfirmation(s string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/n]: ", s)
		response, err := reader.ReadString('\n')
		if err != nil {
			logrus.Fatal(err)
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
	}
}
