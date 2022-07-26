package settings

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
		return s.Default
	}
	return provider.Get(s.Name)
}

func (s Setting) Set(value string) error {
	if provider == nil {
		return nil
	}
	return provider.Set(s.Name, value)
}
