package utils

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

func SSHPrivateKeyPath(sshKey string) (string, error) {
	if sshKey[:2] == "~/" {
		sshKey = filepath.Join(UserHome(), sshKey[2:])
	}
	buff, err := ioutil.ReadFile(sshKey)
	if err != nil {
		return "", fmt.Errorf("error while reading SSH key file: %v", err)
	}
	return string(buff), nil
}

func GetSSHConfig(username, sshPrivateKeyString string) (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	signer, err := parsePrivateKey(sshPrivateKeyString)
	if err != nil {
		return config, err
	}

	config.Auth = append(config.Auth, ssh.PublicKeys(signer))

	return config, nil
}

func parsePrivateKey(keyBuff string) (ssh.Signer, error) {
	return ssh.ParsePrivateKey([]byte(keyBuff))
}
