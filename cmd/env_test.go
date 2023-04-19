package cmd

import (
	"bytes"
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSubstRunE(t *testing.T) {
	// Set environment variables for testing
	err := os.Setenv("ENV_NAME", "world")
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "substitute variables",
			input:  "Hello $ENV_NAME!",
			output: "Hello world!",
		},
		{
			name:   "no variables to substitute",
			input:  "Hello, world!",
			output: "Hello, world!",
		},
		{
			name:   "empty input",
			input:  "",
			output: "",
		},
		{
			name:   "no variables JSON",
			input:  `{"foo": "bar"}`,
			output: `{"foo": "bar"}`,
		},
		{
			name:   "substitute variables JSON",
			input:  `{"foo": "$ENV_NAME"}`,
			output: `{"foo": "world"}`,
		},
		{
			name:   "no variables key=value",
			input:  `foo=bar`,
			output: `foo=bar`,
		},
	}

	// Run tests for each test case as argument
	for _, tc := range testCases {
		t.Run("arg: "+tc.name, func(t *testing.T) {
			// Set up test command
			cmd := newEnvCmd()

			// Capture output
			outputBuf := bytes.Buffer{}
			cmd.SetOut(&outputBuf)

			// Run command
			cmd.SetArgs([]string{"subst", tc.input})
			err := cmd.Execute()

			// Check output and error
			assert.NilError(t, err)
			assert.Equal(t, tc.output, outputBuf.String())
		})
	}
	// Run tests for each test case as stdin
	for _, tc := range testCases {
		t.Run("stdin: "+tc.name, func(t *testing.T) {
			// Set up test command
			cmd := newEnvCmd()

			// Set up input
			inputBuf := bytes.NewBufferString(tc.input)
			cmd.SetIn(inputBuf)

			// Capture output
			outputBuf := bytes.Buffer{}
			cmd.SetOut(&outputBuf)

			// Run command
			cmd.SetArgs([]string{"subst"})
			err = cmd.Execute()

			// Check output and error
			assert.NilError(t, err)
			assert.Equal(t, tc.output, outputBuf.String())
		})
	}
}
