package sshkey

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cnrancher/autok3s/pkg/common"
	pkgsshkey "github.com/cnrancher/autok3s/pkg/sshkey"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export <name>",
	Args:  cobra.ExactArgs(1),
	Short: "export the specificed ssh key pair to files",
	Long: `export the specificed ssh key pair as files to path. 
private key would be id_rsa, public key would be id_rsa.pub and publci key certificate would be pub.cert`,
	PreRunE: validateFiles,
	Run:     utils.CommandExitWithoutHelpInfo(export),
}

func init() {
	exportCmd.Flags().StringVarP(&sshKeyFlags.OutputPath, "output", "o", ".", "The path to write key pair files, will write to id_rsa, id_rsa.pub and pub.cert under the output path")
}

func validateFiles(_ *cobra.Command, args []string) error {
	if err := pathExists(sshKeyFlags.OutputPath); err != nil {
		return err
	}
	toValidate := []string{}
	name := args[0]
	rtn, err := common.DefaultDB.ListSSHKey(&name)
	if err != nil {
		return err
	}
	if len(rtn) == 0 {
		return fmt.Errorf("ssh key %s doesn't exist", name)
	}
	target := rtn[0]
	checkmap := map[string]string{
		pkgsshkey.PrivateKeyFilename:  target.SSHKey,
		pkgsshkey.PublicKeyFilename:   target.SSHPublicKey,
		pkgsshkey.CertificateFilename: target.SSHCert,
	}
	for filename, toCheck := range checkmap {
		if toCheck == "" {
			continue
		}
		toValidate = append(toValidate, filename)
	}

	return pathsNotExists(sshKeyFlags.OutputPath, toValidate...)
}

func export(cmd *cobra.Command, args []string) error {
	target, err := common.DefaultDB.ListSSHKey(&args[0])
	if err != nil {
		return err
	}
	if err := exportKeyFiles(sshKeyFlags.OutputPath, target[0]); err != nil {
		return err
	}
	cmd.Printf("ssh key %s is written to directory %s\n", target[0].Name, sshKeyFlags.OutputPath)
	return nil
}

func pathsNotExists(basePath string, paths ...string) error {
	for _, path := range paths {
		if basePath != "" {
			path = filepath.Join(basePath, path)
		}
		if err := pathNotExists(path); err != nil {
			return err
		}
	}
	return nil
}

func exportKeyFiles(path string, target *common.SSHKey) error {
	dataMap := map[string]string{
		pkgsshkey.PrivateKeyFilename:  target.SSHKey,
		pkgsshkey.PublicKeyFilename:   target.SSHPublicKey,
		pkgsshkey.CertificateFilename: target.SSHCert,
	}
	for filename, data := range dataMap {
		if data == "" {
			continue
		}
		targetPath := filepath.Join(path, filename)
		if err := os.WriteFile(targetPath, []byte(data), 0600); err != nil {
			return fmt.Errorf("failed to write ssh key pair %s to files, %v", target.Name, err)
		}
	}
	return nil
}
