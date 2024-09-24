package sshkey

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/cnrancher/autok3s/pkg/common"
	pkgsshkey "github.com/cnrancher/autok3s/pkg/sshkey"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/validation"
)

var createCmd = &cobra.Command{
	Use:     "create <name>",
	Short:   "Create a new sshkey pair",
	Args:    cobra.ExactArgs(1),
	PreRunE: validateCreateArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := create(cmd, args); err != nil {
			cmd.PrintErr(err)
			os.Exit(1)
		}
		return nil
	},
}

var flagToQuestion = map[string]*survey.Question{
	"bits": {
		Name: "bits",
		Prompt: &survey.Input{
			Message: "The bits for the SSH private key",
			Default: "2048",
		},
		Validate: validateBits,
	},
	"output": {
		Name: "outputPath",
		Prompt: &survey.Input{
			Message: "The output file directory",
			Default: ".",
		},
		Validate: pathExists,
	},
	"key": {
		Name: "privateKeyPath",
		Prompt: &survey.Input{
			Message: "The private key file path",
		},
		Validate: survey.ComposeValidators(
			survey.Required,
			pathExists,
		),
	},
	"passphrase": {
		Name: "passphrase",
		Prompt: &survey.Password{
			Message: "The passphrase of the private, required by the private key",
		},
	},
	"public-key": {
		Name: "publicKeyPath",
		Prompt: &survey.Input{
			Message: "The public key file path, empty for none",
		},
		Validate: pathExists,
	},
	"cert": {
		Name: "certPath",
		Prompt: &survey.Input{
			Message: "The file path of the ssh certificate signed by CA, empty for none",
		},
		Validate: pathExists,
	},
}

func init() {
	createCmd.Flags().BoolVarP(&sshKeyFlags.Generate, "generate", "g", false, "Generating a new ssh key pair")
	createCmd.Flags().IntVarP(&sshKeyFlags.Bits, "bits", "b", 2048, "The bits for the new generated ssh private key")
	createCmd.Flags().StringVar(&sshKeyFlags.Passphrase, "passphrase", "", "The passphrase of ssh private key if needed")
	createCmd.Flags().StringVarP(&sshKeyFlags.OutputPath, "output", "o", ".", "The path to write key pair files, will write to id_rsa and id_rsa.pub under the output path")
	createCmd.Flags().StringVar(&sshKeyFlags.PrivateKeyPath, "key", "", "The ssh private key path")
	createCmd.Flags().StringVar(&sshKeyFlags.PublicKeyPath, "public-key", "", "The ssh public key path for the ssh private key")
	createCmd.Flags().StringVar(&sshKeyFlags.CertPath, "cert", "", "The ssh certificate signed by CA")
}

func validateCreateArgs(cmd *cobra.Command, args []string) (err error) {
	name := args[0]
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("name is not validated %s, %v", name, errs)
	}

	if exist, _ := common.DefaultDB.SSHKeyExists(name); exist {
		return fmt.Errorf("ssh key %s already exists", name)
	}

	if !cmd.Flags().Changed("generate") && !cmd.Flags().Changed("key") {
		sshKeyFlags.Generate, err = utils.AskForConfirmationWithError("Are you going to generate a new RSA ssh key pair?", true)
		if err != nil {
			return err
		}
	}
	qs := []*survey.Question{}
	// generate key pair and set bits
	if sshKeyFlags.Generate && !cmd.Flags().Changed("generated") {
		qs = getQuestins(cmd, "bits", "output")
	}

	if !sshKeyFlags.Generate && !cmd.Flags().Changed("key") {
		qs = getQuestins(cmd, "key", "public-key", "cert")
	}

	if err = survey.Ask(qs, &sshKeyFlags); err != nil {
		return err
	}

	if sshKeyFlags.Generate {
		if err := validateBits(sshKeyFlags.Bits); err != nil {
			return err
		}
		sshKeyFlags.OutputPath = utils.StripUserHome(sshKeyFlags.OutputPath)
		if err := pathExists(sshKeyFlags.OutputPath); err != nil {
			return err
		}

	} else {
		if sshKeyFlags.PrivateKeyPath == "" {
			return fmt.Errorf("private key is required when not generating new keys")
		}
		for _, toCheck := range []*string{&sshKeyFlags.PrivateKeyPath, &sshKeyFlags.PublicKeyPath, &sshKeyFlags.CertPath} {
			*toCheck = utils.StripUserHome(*toCheck)
			if err := pathExists(*toCheck); err != nil {
				return err
			}
		}
		if needed, err := pkgsshkey.NeedPassword(sshKeyFlags.PrivateKeyPath); err != nil {
			return err
		} else if needed {
			question := flagToQuestion["passphrase"]
			if err := survey.AskOne(question.Prompt, &sshKeyFlags.Passphrase); err != nil {
				return err
			}
		}
	}

	return nil
}

func create(cmd *cobra.Command, args []string) error {
	toSave := common.SSHKey{
		Name:          args[0],
		SSHPassphrase: sshKeyFlags.Passphrase,
		Bits:          sshKeyFlags.Bits,
	}

	if sshKeyFlags.Generate {
		if err := pathsNotExists(sshKeyFlags.OutputPath, pkgsshkey.PrivateKeyFilename, pkgsshkey.PublicKeyFilename); err != nil {
			return err
		}
		infoMsg := fmt.Sprintf("generating RSA ssh key pair with %d bit size", sshKeyFlags.Bits)
		if sshKeyFlags.Passphrase != "" {
			infoMsg += " and passphrase"
		}
		cmd.Print(infoMsg + "...\n")

		if err := pkgsshkey.GenerateSSHKey(&toSave); err != nil {
			return err
		}

		cmd.Printf("ssh key %s generated\n", toSave.Name)

		if err := exportKeyFiles(sshKeyFlags.OutputPath, &toSave); err != nil {
			return fmt.Errorf("ssh key pair saved but got error: %v", err)
		}
		cmd.Printf("ssh key %s is written to directory %s\n", toSave.Name, sshKeyFlags.OutputPath)

		return nil
	}

	fileContentMap := map[*string]string{
		&toSave.SSHKey:       sshKeyFlags.PrivateKeyPath,
		&toSave.SSHPublicKey: sshKeyFlags.PublicKeyPath,
		&toSave.SSHCert:      sshKeyFlags.CertPath,
	}

	for pointer, path := range fileContentMap {
		if path == "" {
			continue
		}
		content, err := utils.GetFileContent(path)
		if err != nil {
			return err
		}
		*pointer = string(content)
	}
	if err := pkgsshkey.CreateSSHKey(&toSave); err != nil {
		return err
	}
	cmd.Printf("ssh key %s loaded\n", toSave.Name)
	return nil
}

func getQuestins(cmd *cobra.Command, flags ...string) []*survey.Question {
	rtn := []*survey.Question{}
	for _, f := range flags {
		if _, ok := flagToQuestion[f]; ok && !cmd.Flags().Changed(f) {
			rtn = append(rtn, flagToQuestion[f])
		}
	}
	return rtn
}

func validateBits(ans interface{}) error {
	var v int
	var err error
	switch value := ans.(type) {
	case string:
		v, err = strconv.Atoi(value)
	case int:
		v = value
	}
	if err != nil {
		return err
	}
	if v%256 != 0 {
		err = errors.New("the bits(RSA key size) must be a multiple of 256")
	}

	return err
}

func pathExists(val interface{}) error {
	v, ok := val.(string)
	if !ok {
		v = fmt.Sprintf("%v", val)
	}
	if v == "" {
		return nil
	}
	_, err := os.Stat(utils.StripUserHome(v))
	return err
}

func pathNotExists(val interface{}) error {
	path, _ := val.(string)
	if err := pathExists(val); err == nil {
		return fmt.Errorf("file already %s exists", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}
