package addon

var (
	addonFlags = flags{}
)

type flags struct {
	Name        string
	Description string
	FromFile    string
	Manifest    string
	Values      map[string]string
}
