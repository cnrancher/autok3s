package sshkey

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var (
	listCmd = &cobra.Command{
		Use:     "list [name]",
		Aliases: []string{"ls"},
		Short:   "List all stored ssh key pairs.",
		Args:    cobra.MaximumNArgs(1),
		Run:     utils.CommandExitWithoutHelpInfo(list),
	}
)

func init() {
	listCmd.Flags().BoolVarP(&sshKeyFlags.isJSON, "json", "j", sshKeyFlags.isJSON, "json output")
}

func list(cmd *cobra.Command, args []string) error {
	var err error
	var list []*common.SSHKey
	if len(args) == 1 {
		list, err = common.DefaultDB.ListSSHKey(&args[0])
	} else {
		list, err = common.DefaultDB.ListSSHKey(nil)
	}

	if err != nil {
		return err
	}
	if sshKeyFlags.isJSON {
		data, err := json.Marshal(list)
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
	table.SetHeader([]string{"Name", "Encrypted"})
	for _, key := range list {
		table.Append([]string{
			key.Name,
			fmt.Sprintf("%v", key.HasPassword),
		})
	}
	table.Render()
	return nil
}
