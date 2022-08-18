package common

import (
	"fmt"

	"github.com/cnrancher/autok3s/pkg/types"

	"gorm.io/gorm"
)

type State string

var (
	// PackageActive is the state after package downloaded
	PackageActive State = "Active"
	// PackageOutOfSync is the state when downloading package fails
	PackageOutOfSync State = "OutOfSync"
)

type Package struct {
	Name       string            `json:"name,omitempty" gorm:"primaryKey;->;<-:create" wrangler:"required,noupdate"`
	K3sVersion string            `json:"k3sVersion,omitempty" wrangler:"required"`
	Archs      types.StringArray `json:"archs,omitempty" gorm:"type:text" wrangler:"required"`
	FilePath   string            `json:"filePath,omitempty" wrangler:"nocreate,noupdate"`
	State      State             `json:"state,omitempty" wrangler:"nocreate,noupdate"`
}

func (p Package) GetID() string {
	return p.Name
}

func (s *Store) ListPackages(name *string) ([]Package, error) {
	var rtn []Package
	var singleRtn Package
	var rtnDB *gorm.DB
	if name == nil {
		rtnDB = s.DB.Find(&rtn)
	} else {
		rtnDB = s.DB.Model(Package{}).First(&singleRtn, "name = ?", *name)
	}
	if rtnDB.Error != nil {
		return nil, rtnDB.Error
	}
	if name != nil {
		rtn = append(rtn, singleRtn)
	}
	return rtn, nil
}

func (s *Store) SavePackage(pkg Package) error {
	return s.DB.Save(pkg).Error
}

func (s *Store) DeletePackage(name string) error {
	pkg, err := s.ListPackages(&name)
	if err == gorm.ErrRecordNotFound {
		return fmt.Errorf("package name %s not found", name)
	}
	if err != nil {
		return err
	}
	return s.DB.Delete(&pkg[0]).Error
}

// package exists will return error if not exists or error
func (s *Store) PackageExists(name string) error {
	_, err := s.ListPackages(&name)
	return err
}
