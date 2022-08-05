package airgap

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	listCmd = &cobra.Command{
		Use:   "ls",
		Short: "List all stored airgap packages.",
		RunE:  list,
	}
)

func init() {
	listCmd.Flags().BoolVarP(&airgapFlags.isJSON, "json", "j", airgapFlags.isJSON, "json output")
}

func list(cmd *cobra.Command, args []string) error {
	pkgs, err := common.DefaultDB.ListPackages(nil)
	if err != nil {
		return errors.Wrap(err, "failed to list airgap packages.")
	}
	if airgapFlags.isJSON {
		data, err := json.Marshal(pkgs)
		if err != nil {
			return err
		}
		cmd.Printf("%s\n", string(data))
		return nil
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeader([]string{"Name", "K3sVersion", "Archs", "State"})
	for _, pkg := range pkgs {
		table.Append([]string{
			pkg.Name,
			pkg.K3sVersion,
			strings.Join(pkg.Archs, ","),
			string(pkg.State),
		})
	}
	table.Render()
	return nil
}
