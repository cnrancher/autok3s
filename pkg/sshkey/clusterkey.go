package sshkey

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/pkg/errors"
)

const (
	PrivateKeyFilename  = "id_rsa"
	PublicKeyFilename   = "id_rsa.pub"
	CertificateFilename = "pub.cert"
)

func StoreClusterSSHKeys(clusterName string, ssh *types.SSH) (*types.SSH, error) {
	var rtn *types.SSH
	if !NeedSSHKeys(*ssh) {
		return nil, nil
	}
	base := common.GetClusterContextPath(clusterName)
	// copy key from stored ssh key pair
	if ssh.SSHKeyName != "" && ssh.SSHCertPath == "" && ssh.SSHKeyPath == "" {
		keys, err := common.DefaultDB.ListSSHKey(&ssh.SSHKeyName)
		if err != nil {
			return nil, err
		}
		if len(keys) != 1 {
			return nil, fmt.Errorf("failed to get ssh key %s from db", ssh.SSHKeyName)
		}
		key := keys[0]
		rtn = ssh
		rtn.SSHKey = key.SSHKey
		rtn.SSHCert = key.SSHCert
	}

	// save cert to cert path
	if ssh.SSHCertPath == "" && ssh.SSHCert != "" {
		if rtn == nil {
			rtn = ssh
		}
		certPath := filepath.Join(base, CertificateFilename)
		if err := os.RemoveAll(certPath); err != nil {
			return nil, err
		}
		if err := os.WriteFile(certPath, []byte(ssh.SSHCert), 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to write cluster ssh cert to file %s", certPath)
		}
		rtn.SSHCertPath = certPath
	}

	// save key to key path
	if ssh.SSHKeyPath == "" && ssh.SSHKey != "" {
		if rtn == nil {
			rtn = ssh
		}
		keyPath := filepath.Join(base, PrivateKeyFilename)
		if err := os.RemoveAll(keyPath); err != nil {
			return nil, err
		}
		if err := os.WriteFile(keyPath, []byte(ssh.SSHKey), 0644); err != nil {
			return nil, errors.Wrapf(err, "failed to write cluster ssh private key to file %s", keyPath)
		}
		rtn.SSHKeyPath = keyPath
	}
	return rtn, nil
}

func NeedSSHKeys(ssh types.SSH) bool {
	return !ssh.SSHAgentAuth && ssh.SSHPassword == ""
}
