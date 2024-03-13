package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
)

func TestLocalPipelineValues(t *testing.T) {
	tests := []struct {
		name       string
		parameters Parameters
		wantKeys   []string
	}{
		{
			name:       "standard values given nil parameters",
			parameters: nil,
			wantKeys: []string{
				"pipeline.id",
				"pipeline.number",
				"pipeline.project.git_url",
				"pipeline.project.type",
				"pipeline.git.tag",
				"pipeline.git.branch",
				"pipeline.git.revision",
				"pipeline.git.base_revision",
			},
		},
		{
			name:       "standard and prefixed parameters given map of parameters",
			parameters: Parameters{"foo": "bar", "baz": "buzz"},
			wantKeys: []string{
				"pipeline.id",
				"pipeline.number",
				"pipeline.project.git_url",
				"pipeline.project.type",
				"pipeline.git.tag",
				"pipeline.git.branch",
				"pipeline.git.revision",
				"pipeline.git.base_revision",
				"pipeline.parameters.foo",
				"pipeline.parameters.baz",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatchf(t, tt.wantKeys, maps.Keys(LocalPipelineValues(tt.parameters)), "LocalPipelineValues(%v)", tt.parameters)
		})
	}
}
