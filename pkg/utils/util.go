package utils

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/sirupsen/logrus"
)

const (
	tmpl = `
{{- range $key, $value := .}}
  - {{$key}}: {{$value}}
{{- end}}
`
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

func AskForSelectItem(s string, ss map[string]string) string {
	reader := bufio.NewReader(os.Stdin)
	t := template.New("tmpl")
	t, err := t.Parse(tmpl)
	if err != nil {
		return ""
	}
	buffer := new(bytes.Buffer)
	if err := t.Execute(buffer, ss); err != nil {
		return ""
	}
	fmt.Printf("%s: \n \t%s\n[choose one id]: ", s, buffer.String())
	response, err := reader.ReadString('\n')
	if err != nil {
		logrus.Fatal(err)
	}
	return ss[strings.ToLower(strings.TrimSpace(response))]
}
