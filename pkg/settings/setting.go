package settings

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
)

type Setting struct {
	Name        string
	Default     string
	Description string
}

type Provider interface {
	Get(name string) string
	Set(name, value string) error
	SetIfUnset(name, value string) error
	SetAll(settings map[string]Setting) error
}

var (
	settings        = map[string]Setting{}
	provider        Provider
	WhitelistDomain = newSetting("whitelist-domain", "", "the domains or ips which allowed in autok3s UI proxy")
	EnableMetrics   = newSetting("enable-metrics", "promote", "Should enable telemetry or not")
	InstallUUID     = newSetting("install-uuid", "", "The autok3s instance unique install id")

	InstallScript         = newSetting("install-script", "", "The k3s offline install script with base64 encode")
	ScriptUpdateSource    = newSetting("install-script-source-repo", "https://rancher-mirror.rancher.cn/k3s/k3s-install.sh", "The install script auto update source, github or aliyun oss")
	PackageDownloadSource = newSetting("package-download-source", "github", "The airgap package download source, github and aliyunoss are validated.")

	HelmDashboardEnabled = newSetting("helm-dashboard-enabled", "false", "The helm-dashboard is enabled or not")
	HelmDashboardPort    = newSetting("helm-dashboard-port", "", "The helm-dashboard server port after enabled")
)

func newSetting(
	name, def, desc string,
) Setting {
	s := Setting{
		Name:        name,
		Default:     def,
		Description: desc,
	}
	settings[name] = s
	return s
}

// SetProvider will set the given provider as the global provider for all settings
func SetProvider(p Provider) error {
	if err := p.SetAll(settings); err != nil {
		return err
	}
	provider = p
	return nil
}

func (s Setting) Get() string {
	if provider == nil {
		key := GetEnvKey(s.Name)
		envValue := os.Getenv(key)
		if envValue != "" {
			return envValue
		}
		return settings[s.Name].Default
	}
	return provider.Get(s.Name)
}

func (s Setting) Set(value string) error {
	if provider == nil {
		setting := settings[s.Name]
		setting.Default = value
		settings[s.Name] = setting
		return nil
	}
	return provider.Set(s.Name, value)
}

func GetScriptFromSource(writer io.Writer) error {
	sourceURL := ScriptUpdateSource.Get()
	if _, err := url.Parse(sourceURL); err != nil {
		return errors.Wrap(err, "install script source url is not validated")
	}

	resp, err := http.Get(sourceURL)
	if err != nil {
		return errors.Wrap(err, "failed to make request to install script source url")
	}
	defer resp.Body.Close()

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return errors.Wrap(err, "failed to copy data to target writer")
	}
	return nil
}

// GetEnvKey will return the given string formatted as a autok3s environmental variable.
func GetEnvKey(key string) string {
	return "AUTOK3S_" + strings.ToUpper(strings.Replace(key, "-", "_", -1))
}
