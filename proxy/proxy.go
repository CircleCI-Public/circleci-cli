package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Exec will invoke the given command and proxy any arguments for backwards compatibility.
func Exec(command []string, args []string) error {
	agent, err := exec.LookPath("circleci-agent")
	if err != nil {
		return fmt.Errorf("please ensure that circleci-agent is installed, expected this to be called inside a job: %w", err)
	}

	arguments := append([]string{agent}, command...)
	arguments = append(arguments, args...)

	err = syscall.Exec(agent, arguments, os.Environ()) // #nosec
	if err != nil {
		return fmt.Errorf("failed to proxy command %s, expected this to be called inside a job: %w", command, err)
	}
	return nil
}
