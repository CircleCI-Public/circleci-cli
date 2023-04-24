package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var TestErrorCase = `
jobs:
	job:
		machine: circleci/classic:201710-01
`

var TestHappyCase = `
jobs:
    job:
        image: non/deprecated
`

func TestDeprecatedImageCheck(t *testing.T) {
	t.Run("happy path - tests deprecated image check works", func(t *testing.T) {
		err := deprecatedImageCheck(&ConfigResponse{
			OutputYaml: TestErrorCase,
		})
		assert.Error(t, err)
	})

	t.Run("happy path - no error if image used", func(t *testing.T) {
		err := deprecatedImageCheck(&ConfigResponse{
			OutputYaml: TestHappyCase,
		})
		assert.Nil(t, err)
	})
}
