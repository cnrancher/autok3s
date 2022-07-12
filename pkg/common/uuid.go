package common

import (
	"github.com/pborman/uuid"
)

const (
	uuidSettingName = "install-uuid"
)

var (
	//assuming uuid is not changed
	uuidCache string
)

func SetupNewInstall() (er error) {
	setting, err := DefaultDB.GetSetting(uuidSettingName)
	if err != nil {
		return err
	}
	if setting.Value != "" {
		return nil
	}
	setting.Value = uuid.NewRandom().String()
	defer func(id string) {
		if er == nil {
			uuidCache = id
		}
	}(setting.Value)
	return DefaultDB.SaveSetting(setting)
}

func GetUUID() (string, error) {
	if uuidCache != "" {
		return uuidCache, nil
	}
	setting, err := DefaultDB.GetSetting(uuidSettingName)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}
