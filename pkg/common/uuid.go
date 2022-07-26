package common

import (
	"github.com/cnrancher/autok3s/pkg/settings"

	"github.com/pborman/uuid"
)

var (
	//assuming uuid is not changed
	uuidCache string
)

func SetupNewInstall() (er error) {
	installUUID := settings.InstallUUID.Get()
	if installUUID != "" {
		uuidCache = installUUID
		return nil
	}

	installUUID = uuid.NewRandom().String()
	defer func(id string) {
		if er == nil {
			uuidCache = id
		}
	}(installUUID)
	return settings.InstallUUID.Set(installUUID)
}

func GetUUID() string {
	if uuidCache != "" {
		return uuidCache
	}
	return settings.InstallUUID.Get()
}
