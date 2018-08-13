package proxy

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
)

// Exec will invoke the given command and proxy any arguments for backwards compatibility.
func Exec(command []string, args []string) error {
	agent, err := exec.LookPath("circleci-agent")
	if err != nil {
		return errors.Wrap(err, "Could not find `picard` executable on $PATH; please ensure that build-agent is installed")
	}

	arguments := append([]string{agent}, command...)
	arguments = append(arguments, args...)

	err = syscall.Exec(agent, arguments, os.Environ()) // #nosec
	return errors.Wrapf(err, "failed to proxy command %s, expected this to be called inside a job", command)
}
