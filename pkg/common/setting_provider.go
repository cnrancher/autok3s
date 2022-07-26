package common

import (
	"github.com/cnrancher/autok3s/pkg/settings"

	"github.com/sirupsen/logrus"
)

type DBSettingProvider struct{}

func (p *DBSettingProvider) Get(name string) string {
	s, err := DefaultDB.GetSetting(name)
	if err != nil {
		logrus.Errorf("failed to get setting %s, %v", name, err)
		return ""
	}
	if s == nil {
		return ""
	}
	return s.Value
}

func (p *DBSettingProvider) Set(name, value string) error {
	return DefaultDB.SaveSetting(&Setting{name, value})
}

func (p *DBSettingProvider) SetIfUnset(name, value string) error {
	s, err := DefaultDB.GetSetting(name)
	if err != nil {
		return err
	}
	if s != nil && s.Value != "" {
		return nil
	}
	return p.Set(name, value)
}

func (p *DBSettingProvider) SetAll(settings map[string]settings.Setting) error {
	for _, s := range settings {
		if err := p.SetIfUnset(s.Name, s.Default); err != nil {
			return err
		}
	}
	return nil
}
