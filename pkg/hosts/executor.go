package hosts

type Script interface {
	ExecuteCommands(cmds ...string) (output string, err error)
	Close() error
}
