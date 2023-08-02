package main

import (
	"os"

	"github.com/CircleCI-Public/circleci-cli/cmd"
)

func main() {
	// See cmd/root.go for Execute()
	if err := cmd.Execute(); err != nil {
		os.Exit(-1)
	}
}
