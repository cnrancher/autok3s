package viper

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

var bindPrefix = "autok3s.providers.%s.%s"

func GetString(p, f string) string {
	return viper.GetString(fmt.Sprintf(bindPrefix, strings.ToLower(p), strings.ToLower(f)))
}
