package cmd

import (
	"time"

	"github.com/CircleCI-Public/circleci-cli/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Orb unit tests", func() {
	Describe("Orb formatters", func() {
		DescribeTable(
			"parameterDefaultToString",
			func(input api.OrbElementParameter, expected string) {
				Expect(parameterDefaultToString(input)).To(Equal(expected))
			},
			Entry(
				"Normal behaviour for string",
				api.OrbElementParameter{
					Type:        "string",
					Description: "",
					Default:     "Normal behavior",
				},
				" (default: 'Normal behavior')",
			),
			Entry(
				"Normal behaviour for enum",
				api.OrbElementParameter{
					Type:        "enum",
					Description: "",
					Default:     "Normal behavior",
				},
				" (default: 'Normal behavior')",
			),
			Entry(
				"Normal behaviour for boolean",
				api.OrbElementParameter{
					Type:        "boolean",
					Description: "",
					Default:     true,
				},
				" (default: 'true')",
			),
			Entry(
				"String value for boolean",
				api.OrbElementParameter{
					Type:        "boolean",
					Description: "",
					Default:     "yes",
				},
				" (default: 'yes')",
			),
			Entry(
				"Time value for string",
				api.OrbElementParameter{
					Type:        "string",
					Description: "",
					Default:     time.Date(2023, 02, 20, 11, 9, 0, 0, time.Now().UTC().Location()),
				},
				" (default: '2023-02-20 11:09:00 +0000 UTC')",
			),
		)
	})
})
