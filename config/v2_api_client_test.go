package config

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestConfigErrorsAsError(t *testing.T) {
	err := configErrorsAsError([]ConfigError{
		{Message: "error on line 1"},
		{Message: "error on line 42"},
	})
	assert.Error(t, err, `config compilation contains errors:
	- error on line 1
	- error on line 42`)
}

func TestConfigErrorsAsError_MultilineMessage(t *testing.T) {
	// A single ConfigError's Message may itself span multiple
	// newline-separated lines; every line should get its own bullet rather
	// than only the first.
	err := configErrorsAsError([]ConfigError{
		{Message: "error on line 1\nerror on line 42"},
	})
	assert.Error(t, err, `config compilation contains errors:
	- error on line 1
	- error on line 42`)
}
