package airgap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pkgairgap "github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var (
	exportCmd = &cobra.Command{
		Use:   "export <name> <path>",
		Short: "export package to a tar.gz file, path can be a specific filename or a directory.",
		Args:  cobra.ExactArgs(2),
		RunE:  export,
	}
	errPathInvalid = errors.New("path should be an existing directory or a file with .tar.gz/tgz suffix")
)

func export(cmd *cobra.Command, args []string) error {
	name := args[0]
	pkgs, err := common.DefaultDB.ListPackages(&name)
	if err == gorm.ErrRecordNotFound {
		return fmt.Errorf("package %s not found", name)
	}
	if err != nil {
		return err
	}
	targetPackage := pkgs[0]
	if targetPackage.State != common.PackageActive {
		return fmt.Errorf("package %s is not active, airgap resources maybe missing", name)
	}

	path := args[1]

	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// check file name if path not exist.
	if os.IsNotExist(err) {
		base := filepath.Base(path)
		if !strings.HasSuffix(base, ".tgz") &&
			!strings.HasSuffix(base, ".tar.gz") {
			return errPathInvalid
		}
		if _, err := os.Lstat(filepath.Dir(path)); err != nil {
			return err
		}
		// here means path is a file with tar.gz/tgz suffix and parent dir exists
	} else if !info.IsDir() {
		// here means that input path is a regular file and should return error
		return errPathInvalid
	} else {
		// here means that the input path is a dir and will use package name as the output name
		path = filepath.Join(path, name+".tar.gz")
	}

	if err := pkgairgap.TarAndGzip(targetPackage.FilePath, path); err != nil {
		return err
	}
	cmd.Printf("package %s export to %s succeed\n", name, path)
	return nil
}
