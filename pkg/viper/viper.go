package viper

import (
	"fmt"
	"strings"

	"github.com/Jason-ZW/autok3s/pkg/common"

	"github.com/spf13/viper"
)

func GetString(p, f string) string {
	return viper.GetString(fmt.Sprintf(common.BindPrefix, strings.ToLower(p), strings.ToLower(f)))
}
