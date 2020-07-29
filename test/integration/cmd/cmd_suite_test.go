package cmd_test

import (
	"testing"

	"github.com/Jason-ZW/autok3s/cmd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cmd Suite")
}

var _ = BeforeSuite(func() {
	rootCmd = cmd.Command()
	rootCmd.AddCommand(
		cmd.CompletionCommand(),
		cmd.VersionCommand("", "", "", ""),
		cmd.GetCommand(),
		cmd.CreateCommand())

	Expect(rootCmd).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	rootCmd = nil
})
