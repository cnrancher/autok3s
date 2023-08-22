package addon

var (
	addonFlags = flags{
		Values:       map[string]string{},
		RemoveValues: []string{},
	}
)

type flags struct {
	Name         string
	Description  string
	FromFile     string
	Values       map[string]string
	RemoveValues []string
}
