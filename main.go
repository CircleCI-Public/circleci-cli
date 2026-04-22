package main

import (
	"errors"
	"os"

	"github.com/CircleCI-Public/circleci-cli/cmd"
	cmdjob "github.com/CircleCI-Public/circleci-cli/cmd/job"
)

func main() {
	// See cmd/root.go for Execute()
	if err := cmd.Execute(); err != nil {
		if errors.Is(err, cmdjob.ErrTestsFailed) {
			os.Exit(1)
		}
		os.Exit(-1)
	}
}
