package utils

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"
)

// Borrowed from https://github.com/AliyunContainerService/docker-machine-driver-aliyunecs/blob/master/aliyunecs/utils.go#L38
const digitals = "0123456789"
const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const specialChars = "()`~!@#$%^&*-+=|{}[]:;'<>,.?/"
const dictionary = digitals + alphabet + specialChars
const paswordLen = 16

func RandomPassword() string {
	var bytes = make([]byte, paswordLen)
	rand.Read(bytes)
	for k, v := range bytes {
		var ch byte

		switch k {
		case 0:
			ch = alphabet[v%byte(len(alphabet))]
		case 1:
			ch = digitals[v%byte(len(digitals))]
		case 2:
			ch = specialChars[v%byte(len(specialChars))]
		default:
			ch = dictionary[v%byte(len(dictionary))]
		}
		bytes[k] = ch
	}
	return string(bytes)
}

func IsExistedNodes(nodes []types.Node, instance string) (int, bool) {
	for index, n := range nodes {
		if n.InstanceID == instance {
			return index, true
		}
	}

	return -1, false
}

func CreateKeyPair(ssh *types.SSH, providerName, name, keypair string) (string, error) {
	var keyPath string
	if ssh.SSHKeyPath == "" && keypair == "" {
		fmt.Printf("[%s] generate default key-pair \n", providerName)
		if err := utils.GenerateSSHKey(common.GetDefaultSSHKeyPath(name, providerName)); err != nil {
			return "", err
		}
		keyPath = common.GetDefaultSSHKeyPath(name, providerName)
	} else {
		keyPath = ssh.SSHKeyPath
		if keypair != "" {
			fmt.Printf("[%s] Using existing key pair %s \n", providerName, keypair)
			return "", nil
		}
	}

	ssh.SSHKeyPath = keyPath
	publicKey, err := ioutil.ReadFile(keyPath + ".pub")
	if err != nil {
		return "", err
	}

	return string(publicKey), nil
}
