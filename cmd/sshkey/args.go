package sshkey

var (
	sshKeyFlags = flags{}
)

type flags struct {
	Generate       bool
	Bits           int
	Passphrase     string
	PrivateKeyPath string
	PublicKeyPath  string
	CertPath       string
	OutputPath     string

	isJSON  bool
	isForce bool
}
