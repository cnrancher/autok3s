package cmd_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/cnrancher/autok3s/pkg/types"

	exec "github.com/alexellis/go-execute/pkg/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	rootDir string
	execCmd string
)

var _ = Describe("Cmd Version Test", func() {
	Context("Long version is correct", func() {
		It("return the correct version info", func() {
			cmd := exec.ExecTask{
				Command:     execCmd,
				Args:        []string{"version"},
				StreamStdio: false,
			}

			res, err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(res.ExitCode).To(Equal(0))

			version := &types.VersionInfo{}
			err = json.Unmarshal([]byte(res.Stdout[8:]), version)
			Expect(err).NotTo(HaveOccurred())
			Expect(version.GitVersion).NotTo(BeEmpty())
			Expect(version.GitCommit).NotTo(BeEmpty())
		})
	})

	Context("Short version is correct", func() {
		It("return the correct version info", func() {
			cmd := exec.ExecTask{
				Command:     execCmd,
				Args:        []string{"version", "-s"},
				StreamStdio: false,
			}

			res, err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())
			Expect(res.ExitCode).To(Equal(0))
			Expect(res.Stdout).NotTo(BeEmpty())
		})
	})
})

func init() {
	rootDir, _ = filepath.Abs(filepath.Join(filepath.Dir("."), "..", "..", ".."))
	execCmd = fmt.Sprintf("%s/bin/autok3s_%s_%s", rootDir, runtime.GOOS, runtime.GOARCH)
}
