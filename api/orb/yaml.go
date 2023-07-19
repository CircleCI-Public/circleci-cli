package orb

import (
	"io"
	"os"

	"github.com/pkg/errors"
)

func loadYaml(path string) (string, error) {
	var err error
	var config []byte
	if path == "-" {
		config, err = io.ReadAll(os.Stdin)
	} else {
		config, err = os.ReadFile(path)
	}

	if err != nil {
		return "", errors.Wrapf(err, "Could not load config file at %s", path)
	}

	return string(config), nil
}
