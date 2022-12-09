package sshkey

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"

	"golang.org/x/crypto/ssh"
)

func NeedPassword(keypath string) (bool, error) {
	content, err := utils.GetFileContent(keypath)
	if err != nil {
		return false, err
	}

	if _, err := ssh.ParseRawPrivateKey(content); err != nil {
		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func CreateSSHKey(key *common.SSHKey, passphrase string) error {
	var err error
	var publicKey ssh.PublicKey
	var privateKey ssh.Signer
	if passphrase != "" {
		privateKey, err = ssh.ParsePrivateKeyWithPassphrase([]byte(key.SSHKey), []byte(passphrase))
		key.HasPassword = true
	} else {
		privateKey, err = ssh.ParsePrivateKey([]byte(key.SSHKey))
	}
	if err != nil {
		return fmt.Errorf("failed to parse private key file %v", err)
	}

	if key.SSHPublicKey != "" {
		publicKey, _, _, _, err = ssh.ParseAuthorizedKey([]byte(key.SSHPublicKey))
		if err != nil {
			return fmt.Errorf("failed to parse public key file %v", err)
		}
		target := privateKey.PublicKey().Marshal()
		source := publicKey.Marshal()
		if !bytes.Equal(target, source) {
			return fmt.Errorf("the ssh public key and private key not matched")
		}
	}
	if key.SSHCert != "" {
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key.SSHCert))
		if err != nil {
			return fmt.Errorf("failed to parse certificate to ssh authorized key file")
		}
		if _, ok := key.(*ssh.Certificate); !ok {
			return fmt.Errorf("failed to parse certificate file to ssh certificate")
		}
	}
	return common.DefaultDB.SaveSSHKey(*key)
}

func GenerateSSHKey(key *common.SSHKey, passphrase string, bits int) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	if passphrase != "" {
		block, err = x509.EncryptPEMBlock(rand.Reader, block.Type, block.Bytes, []byte(passphrase), x509.PEMCipherAES256)
		if err != nil {
			return err
		}
		key.HasPassword = true
	}
	key.SSHKey = string(pem.EncodeToMemory(block))

	publicKey := privateKey.Public()
	sshPublicKey, err := ssh.NewPublicKey(publicKey.(*rsa.PublicKey))
	if err != nil {
		return err
	}

	key.SSHPublicKey = string(ssh.MarshalAuthorizedKey(sshPublicKey))

	return common.DefaultDB.SaveSSHKey(*key)
}
