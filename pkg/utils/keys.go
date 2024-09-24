package utils

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"

	gossh "golang.org/x/crypto/ssh"
)

// This file Borrowed from https://github.com/docker/machine/blob/master/libmachine/ssh/keys.go.
var (
	ErrKeyGeneration     = errors.New("unable to generate key")
	ErrValidation        = errors.New("unable to validate key")
	ErrPublicKey         = errors.New("unable to convert public key")
	ErrUnableToWriteFile = errors.New("unable to write file")
)

// KeyPair struct for private/public key.
type KeyPair struct {
	PrivateKey []byte
	PublicKey  []byte
}

// NewKeyPair generates a new SSH keypair
// This will return a private & public key encoded as DER.
func NewKeyPair() (keyPair *KeyPair, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, ErrKeyGeneration
	}

	if err := priv.Validate(); err != nil {
		return nil, ErrValidation
	}

	privDer := x509.MarshalPKCS1PrivateKey(priv)

	pubSSH, err := gossh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, ErrPublicKey
	}

	return &KeyPair{
		PrivateKey: privDer,
		PublicKey:  gossh.MarshalAuthorizedKey(pubSSH),
	}, nil
}

// WriteToFile writes keypair to files.
func (kp *KeyPair) WriteToFile(privateKeyPath string, publicKeyPath string) error {
	files := []struct {
		File  string
		Type  string
		Value []byte
	}{
		{
			File:  privateKeyPath,
			Value: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Headers: nil, Bytes: kp.PrivateKey}),
		},
		{
			File:  publicKeyPath,
			Value: kp.PublicKey,
		},
	}

	for _, v := range files {
		// ensure folder exist.
		baseDir := path.Dir(v.File)
		if _, err := os.Stat(baseDir); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(baseDir, 0700); err != nil {
					return fmt.Errorf("fail to create default ssh dir: %s", err)
				}
			}
		}
		f, err := os.Create(v.File)
		if err != nil {
			return ErrUnableToWriteFile
		}

		if _, err := f.Write(v.Value); err != nil {
			return ErrUnableToWriteFile
		}

		// windows does not support chmod
		switch runtime.GOOS {
		case "darwin", "freebsd", "linux", "openbsd":
			if err := f.Chmod(0600); err != nil {
				return err
			}
		}
	}

	return nil
}

// Fingerprint calculates the fingerprint of the public key.
func (kp *KeyPair) Fingerprint() string {
	b, _ := base64.StdEncoding.DecodeString(string(kp.PublicKey))
	h := md5.New()

	_, _ = h.Write(b)

	return fmt.Sprintf("%x", h.Sum(nil))
}

// GenerateSSHKey generates SSH keypair based on path of the private key
// The public key would be generated to the same path with ".pub" added.
func GenerateSSHKey(path string) error {
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("desired directory for SSH keys does not exist: %s", err)
		}

		kp, err := NewKeyPair()
		if err != nil {
			return fmt.Errorf("error generating key pair: %s", err)
		}

		if err := kp.WriteToFile(path, fmt.Sprintf("%s.pub", path)); err != nil {
			return fmt.Errorf("error writing keys to file(s): %s", err)
		}
	}

	return nil
}

// RemoveSSHKey delete SSH keypair based on path.
func RemoveSSHKey(path string) error {
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(path)
	}

	return nil
}
