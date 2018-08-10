package proxy

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
)

func Exec(command string, args []string) error {
	agent, err := exec.LookPath("picard")
	if err != nil {
		return errors.Wrap(err, "Could not find `picard` executable on $PATH; please ensure that build-agent is installed")
	}

	err = syscall.Exec(agent, args, os.Environ()) // #nosec
	return errors.Wrap(err, "failed to execute picard command")
}
