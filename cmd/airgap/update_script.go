package airgap

import (
	"bytes"

	"github.com/cnrancher/autok3s/pkg/settings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	updateScriptCmd = &cobra.Command{
		Use:   "update-install-script",
		Short: "Will update the embed k3s install.sh script.",
		RunE:  updateInstallScript,
	}
)

func updateInstallScript(cmd *cobra.Command, args []string) error {
	buff := bytes.NewBuffer([]byte{})
	if err := settings.GetScriptFromSource(buff); err != nil {
		return err
	}
	if err := settings.InstallScript.Set(buff.String()); err != nil {
		return errors.Wrap(err, "failed to update install script")
	}
	cmd.Printf("update install.sh script from source %s done\n", settings.ScriptUpdateSource.Get())
	return nil
}
