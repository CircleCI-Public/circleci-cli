package cmd

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/api"
)

func TestParameterDefaultToString(t *testing.T) {
	tests := []struct {
		name     string
		input    api.OrbElementParameter
		expected string
	}{
		{
			name: "normal behaviour for string",
			input: api.OrbElementParameter{
				Type:        "string",
				Description: "",
				Default:     "Normal behavior",
			},
			expected: " (default: 'Normal behavior')",
		},
		{
			name: "normal behaviour for enum",
			input: api.OrbElementParameter{
				Type:        "enum",
				Description: "",
				Default:     "Normal behavior",
			},
			expected: " (default: 'Normal behavior')",
		},
		{
			name: "normal behaviour for boolean",
			input: api.OrbElementParameter{
				Type:        "boolean",
				Description: "",
				Default:     true,
			},
			expected: " (default: 'true')",
		},
		{
			name: "string value for boolean",
			input: api.OrbElementParameter{
				Type:        "boolean",
				Description: "",
				Default:     "yes",
			},
			expected: " (default: 'yes')",
		},
		{
			name: "time value for string",
			input: api.OrbElementParameter{
				Type:        "string",
				Description: "",
				Default:     time.Date(2023, 02, 20, 11, 9, 0, 0, time.Now().UTC().Location()),
			},
			expected: " (default: '2023-02-20 11:09:00 +0000 UTC')",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, parameterDefaultToString(tc.input), tc.expected)
		})
	}
}
