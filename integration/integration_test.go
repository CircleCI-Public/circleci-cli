// +build integration

package integration

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega/gexec"
)

func TestUserCanPublishAnOrb(t *testing.T) {
	// First (and perhaps last?) step is to teardown any state
	// that may have been left from previous integration tests.
	clearState(t)

	steps := []struct {
		label  string
		args   []string
		expOut string
	}{
		{
			label:  "Listing orbs should return no orbs in my namespace",
			args:   []string{"orb", "list", "integration-namespace"},
			expOut: "...",
		},
		{
			label:  "Successfully create an orb in my namespace",
			args:   []string{"orb", "create", "integration-namespace/my-orb"},
			expOut: "...",
		},
		{
			label:  "Successfully publish an orb in my namespace",
			args:   []string{"orb", "publish", "version: 2.1", "integration-namespace/my-orb@0.0.1"},
			expOut: "...",
		},
		{
			label:  "Listing orbs should now return exactly one orb",
			args:   []string{"orb", "list", "integration-namespace"},
			expOut: "...",
		},
		{
			label:  "Source of published orb should match",
			args:   []string{"orb", "source", "integration-namespace/my-orb@0.0.1"},
			expOut: "...",
		},
		// more steps to be added ...
	}

	// Reusing this existing library, but this should reflect the latest deployed
	// version of our CLI.
	cli, err := gexec.Build("github.com/CircleCI-Public/circleci-cli")
	if err != nil {
		t.Fatalf("unable to compile cli from source: %s", err.Error())
	}

	for _, s := range steps {
		t.Run(s.label, func(t *testing.T) {
			os.Args = s.args

			// Initialize the command.
			cmd := exec.Command(cli, s.args...)

			// Point stdout to something we can read from.
			var stdOut bytes.Buffer
			cmd.Stdout = &stdOut

			// Run the command
			err := cmd.Run()
			if err != nil {
				t.Fatalf("error running command: %s", err.Error())
			}

			// Assert output is correct.
			output := stdOut.String()
			if !cmp.Equal(output, s.expOut) {
				diff := cmp.Diff(output, s.expOut)
				t.Fatalf("unexpected diff in output: %s", diff)
			}
		})
	}
}

func clearState(t *testing.T) {
	// Not yet implemented.
}
