package common

import (
	"github.com/cnrancher/autok3s/pkg/types"
	apitypes "github.com/rancher/apiserver/pkg/types"
)

type Addon struct {
	Name        string          `json:"name" gorm:"primaryKey;not null" wrangler:"required"`
	Description string          `json:"description,omitempty"`
	Manifest    []byte          `json:"manifest" gorm:"type:bytes" wrangler:"required"`
	Values      types.StringMap `json:"values,omitempty" gorm:"type:stringMap"`
}

func (a Addon) GetID() string {
	return a.Name
}

func (s *Store) SaveAddon(addon *Addon) error {
	existAddon, _ := s.GetAddon(addon.Name)
	if existAddon != nil {
		// update addon
		result := s.DB.Where("name = ? ", addon.Name).Omit("name").Save(addon)
		return result.Error
	}
	result := s.DB.Create(addon)
	return result.Error
}

func (s *Store) GetAddon(name string) (*Addon, error) {
	addon := &Addon{}
	result := s.DB.Model(&Addon{}).First(addon, "name = ? ", name)
	if result.Error != nil {
		return nil, result.Error
	}
	return addon, nil
}

func (s *Store) ListAddon() ([]*Addon, error) {
	list := []*Addon{}
	result := s.DB.Find(&list)
	return list, result.Error
}

func (s *Store) DeleteAddon(name string) error {
	_, err := s.GetAddon(name)
	if err != nil {
		return err
	}
	result := s.DB.Where("name = ? ", name).Delete(&Addon{})
	if result.Error == nil {
		s.broadcaster.Broadcast(&event{
			Name: apitypes.RemoveAPIEvent,
			Object: GetAPIObject(&Addon{
				Name: name,
			}),
		})
	}
	return result.Error
}
