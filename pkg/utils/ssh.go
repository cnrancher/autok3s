package utils

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

func SSHPrivateKeyPath(sshKeyPath string) (string, error) {
	if sshKeyPath[:2] == "~/" {
		sshKeyPath = filepath.Join(UserHome(), sshKeyPath[2:])
	}
	buff, err := ioutil.ReadFile(sshKeyPath)
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
