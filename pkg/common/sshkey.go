package common

import (
	"fmt"

	"gorm.io/gorm"
)

type SSHKey struct {
	Name        string `json:"name" gorm:"primaryKey;not null"`
	GenerateKey bool   `json:"generate-key,omitempty" gorm:"-:all" norman:"writeOnly,noupdate"`
	HasPassword bool   `json:"has-password,omitempty" norman:"readonly"`

	SSHCert      string `json:"ssh-cert,omitempty" yaml:"ssh-cert,omitempty"`
	SSHKey       string `json:"ssh-key,omitempty" yaml:"ssh-key,omitempty" norman:"type=password"`
	SSHPublicKey string `json:"ssh-key-public,omitempty" yaml:"ssh-key-public,omitempty"`
}

func (s *SSHKey) GetID() string {
	return s.Name
}

func (s *Store) SaveSSHKey(sshkey SSHKey) error {
	exists, _ := s.SSHKeyExists(sshkey.Name)
	if exists {
		// update sshkey
		result := s.DB.Where("name = ? ", sshkey.Name).Omit("name").Save(sshkey)
		return result.Error
	}
	// save sshkey
	result := s.DB.Create(sshkey)
	return result.Error
}

func (s *Store) ListSSHKey(name *string) ([]*SSHKey, error) {
	var rtn []*SSHKey
	var singleRtn SSHKey
	var rtnDB *gorm.DB
	if name == nil {
		rtnDB = s.DB.Find(&rtn)
	} else {
		rtnDB = s.DB.Model(&SSHKey{}).First(&singleRtn, "name = ?", *name)
	}
	if rtnDB.Error != nil {
		return nil, rtnDB.Error
	}
	if name != nil {
		rtn = append(rtn, &singleRtn)
	}
	return rtn, nil
}

func (s *Store) DeleteSSHKey(name string) error {
	pkg, err := s.ListSSHKey(&name)
	if err == gorm.ErrRecordNotFound {
		return fmt.Errorf("ssh key name %s not found", name)
	}
	if err != nil {
		return err
	}
	return s.DB.Delete(&pkg[0]).Error
}

// package exists will return error if not exists or error
func (s *Store) SSHKeyExists(name string) (bool, error) {
	rtn, err := s.ListSSHKey(&name)
	return len(rtn) != 0, err
}
