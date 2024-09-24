package utils

import (
	"crypto/rand"
	"os"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/sirupsen/logrus"
)

// Borrowed from https://github.com/AliyunContainerService/docker-machine-driver-aliyunecs/blob/master/aliyunecs/utils.go#L38.
const digital = "0123456789"
const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const specialChars = "()`~!@#$%^&*-+=|{}[]:;'<>,.?/"
const dictionary = digital + alphabet + specialChars
const passwordLen = 16

// RandomPassword generate random password.
func RandomPassword() string {
	var bytes = make([]byte, passwordLen)
	_, _ = rand.Read(bytes)
	for k, v := range bytes {
		var ch byte

		switch k {
		case 0:
			ch = alphabet[v%byte(len(alphabet))]
		case 1:
			ch = digital[v%byte(len(digital))]
		case 2:
			ch = specialChars[v%byte(len(specialChars))]
		default:
			ch = dictionary[v%byte(len(dictionary))]
		}
		bytes[k] = ch
	}
	return string(bytes)
}

// IsExistedNodes check that the node already exists.
func IsExistedNodes(nodes []types.Node, instance string) (int, bool) {
	for index, n := range nodes {
		if n.InstanceID == instance {
			return index, true
		}
	}

	return -1, false
}

// CreateKeyPair create ssh key pair if key path not given.
func CreateKeyPair(ssh *types.SSH, providerName, name, keypair string) ([]byte, error) {
	var keyPath string
	if ssh.SSHKeyPath == "" && keypair == "" {
		logrus.Infof("[%s] generate default key-pair", providerName)
		if err := utils.GenerateSSHKey(common.GetDefaultSSHKeyPath(name, providerName)); err != nil {
			return nil, err
		}
		keyPath = common.GetDefaultSSHKeyPath(name, providerName)
	} else {
		keyPath = ssh.SSHKeyPath
		if keypair != "" {
			logrus.Infof("[%s] use existing key pair %s", providerName, keypair)
			return nil, nil
		}
	}

	ssh.SSHKeyPath = keyPath
	publicKey, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return nil, err
	}

	return publicKey, nil
}
