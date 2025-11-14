package cmd

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/CircleCI-Public/circleci-cli/settings"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("run", func() {
	var (
		tempDir    string
		pluginPath string
		config     *settings.Config
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "circleci-plugin-test")
		Expect(err).ToNot(HaveOccurred())

		// Create a test plugin
		pluginPath = filepath.Join(tempDir, "circleci-test-plugin")
		if runtime.GOOS == "windows" {
			pluginPath = pluginPath + ".bat"
		}
		pluginScript := `#!/bin/bash
echo "Plugin executed"
echo "Args: $@"
exit 0
`
		err = os.WriteFile(pluginPath, []byte(pluginScript), 0755)
		Expect(err).ToNot(HaveOccurred())

		config = &settings.Config{}
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("plugin execution", func() {
		It("should find and execute a plugin in PATH", func() {
			// Add tempDir to PATH
			oldPath := os.Getenv("PATH")
			os.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)
			defer os.Setenv("PATH", oldPath)

			cmd := newRunCommand(config)
			cmd.SetArgs([]string{"test-plugin", "arg1", "arg2"})
			err := cmd.Execute()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error when plugin is not found", func() {
			cmd := newRunCommand(config)
			cmd.SetArgs([]string{"nonexistent-plugin"})
			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("plugin 'nonexistent-plugin' not found"))
		})

		It("should require at least one argument", func() {
			cmd := newRunCommand(config)
			cmd.SetArgs([]string{})
			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("requires at least 1 arg"))
		})
	})
})
