package utils

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const sshAuthSock = "SSH_AUTH_SOCK"

func StripUserHome(path string) string {
	if len(path) > 2 && path[:2] == "~/" {
		path = filepath.Join(UserHome(), path[2:])
	}
	return path
}

func GetFileContent(path string) ([]byte, error) {
	buff, err := os.ReadFile(StripUserHome(path))
	if err != nil {
		return []byte{}, err
	}
	return buff, nil
}

// SSHPrivateKeyPath returns ssh private key content from given path.
func SSHPrivateKeyPath(sshKey string) (string, error) {
	content, err := GetFileContent(sshKey)
	if err != nil {
		return "", fmt.Errorf("error while reading SSH key file: %v", err)
	}
	return string(content), nil
}

// SSHCertificatePath returns ssh certificate key content from given path
func SSHCertificatePath(sshCertPath string) (string, error) {
	content, err := GetFileContent(sshCertPath)
	if err != nil {
		return "", fmt.Errorf("error while reading SSH certificate file: %v", err)
	}
	return string(content), nil
}

// GetSSHConfig generate ssh config.
func GetSSHConfig(username, sshPrivateKeyString, passphrase, sshCert string, password string, timeout time.Duration, useAgentAuth bool) (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{
		User:            username,
		Timeout:         timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if useAgentAuth {
		if sshAgentSock := os.Getenv(sshAuthSock); sshAgentSock != "" {
			sshAgent, err := net.Dial("unix", sshAgentSock)
			if err != nil {
				return config, fmt.Errorf("cannot connect to SSH Auth socket %q: %s", sshAgentSock, err)
			}

			config.Auth = append(config.Auth, ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers))

			logrus.Debugf("using %q SSH_AUTH_SOCK", sshAgentSock)
			return config, nil
		}
	} else if sshPrivateKeyString != "" {
		var (
			signer ssh.Signer
			err    error
		)
		if passphrase != "" {
			signer, err = parsePrivateKeyWithPassphrase(sshPrivateKeyString, passphrase)
		} else {
			signer, err = parsePrivateKey(sshPrivateKeyString)
		}
		if err != nil {
			return config, err
		}

		if len(sshCert) > 0 {
			key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(sshCert))
			if err != nil {
				return config, fmt.Errorf("unable to parse SSH certificate: %v", err)
			}

			if _, ok := key.(*ssh.Certificate); !ok {
				return config, fmt.Errorf("unable to cast public key to SSH certificate")
			}
			signer, err = ssh.NewCertSigner(key.(*ssh.Certificate), signer)
			if err != nil {
				return config, err
			}
		}

		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	} else if password != "" {
		config.Auth = append(config.Auth, ssh.Password(password))
	}

	return config, nil
}

func parsePrivateKey(keyBuff string) (ssh.Signer, error) {
	return ssh.ParsePrivateKey([]byte(keyBuff))
}

func parsePrivateKeyWithPassphrase(keyBuff, passphrase string) (ssh.Signer, error) {
	return ssh.ParsePrivateKeyWithPassphrase([]byte(keyBuff), []byte(passphrase))
}
