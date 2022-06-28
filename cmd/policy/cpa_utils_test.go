package policy

import (
	"fmt"
	"testing"

	"github.com/CircleCI-Public/circle-policy-agent/cpa"
	"gotest.tools/v3/assert"
)

func TestGetPolicyDecisionLocally(t *testing.T) {
	testcases := []struct {
		Name           string
		PolicyPath     string
		Input          string
		ExpectedOutput *cpa.Decision
		ExpectedErr    string
	}{
		{
			Name:        "fails on non-existing policyPath",
			PolicyPath:  "./testdata/does_not_exist",
			ExpectedErr: "failed to get document bundle for path: failed to get path info: ",
		},
		{
			Name:        "fails for empty policy FILE",
			PolicyPath:  "./testdata/test0/empty_file.rego",
			ExpectedErr: "failed to parse policy bundle: failed to parse policy file(s): failed to parse file \"empty_file.rego\": empty_file.rego:0: rego_parse_error: empty module",
		},
		{
			Name:        "fails on bad input",
			PolicyPath:  "./testdata/test1/policy.rego",
			Input:       ":",
			ExpectedErr: "invalid config: yaml: did not find expected key",
		},
		{
			Name:        "fails for policy FILE provided locally where package name is not org",
			PolicyPath:  "./testdata/test.rego",
			ExpectedErr: "failed to make decision: no org policy evaluations found",
		},
		{
			Name:           "successfully performs decision for policy FILE provided locally",
			PolicyPath:     "./testdata/test1/policy.rego",
			Input:          "name: bob",
			ExpectedOutput: &cpa.Decision{Status: "PASS", EnabledRules: []string{"name_is_bob"}},
		},
		{
			Name:       "successfully performs decision for policy FILE provided locally",
			PolicyPath: "./testdata/test1/policy.rego",
			Input:      "name: not_bob",
			ExpectedOutput: &cpa.Decision{Status: "SOFT_FAIL", EnabledRules: []string{"name_is_bob"},
				SoftFailures: []cpa.Violation{{Rule: "name_is_bob", Reason: "name must be bob!"}}},
		},
		{
			Name:       "successfully performs decision for policy DIRECTORY provided locally",
			PolicyPath: "./testdata/test2/policies",
			Input: `
name: "not_bob"
type: "not_person"`,
			ExpectedOutput: &cpa.Decision{Status: "HARD_FAIL", EnabledRules: []string{"name_is_bob", "type_is_person"},
				SoftFailures: []cpa.Violation{{Rule: "name_is_bob", Reason: "name must be bob!"}},
				HardFailures: []cpa.Violation{{Rule: "type_is_person", Reason: "type must be person!"}}},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			decision, err := getPolicyDecisionLocally(tc.PolicyPath, tc.Input)

			if tc.ExpectedErr == "" {
				assert.NilError(t, err)
				fmt.Println(tc.ExpectedOutput)
				assert.DeepEqual(t, tc.ExpectedOutput, decision)
			} else {
				assert.ErrorContains(t, err, tc.ExpectedErr)
			}
		})
	}
}

func TestGetDocumentBundleFromPath(t *testing.T) {
	testcases := []struct {
		Name           string
		PolicyPath     string
		ExpectedOutput map[string]string
		ExpectedErr    string
	}{
		{
			Name:        "fails on non-existing policyPath",
			PolicyPath:  "./testdata/does_not_exist",
			ExpectedErr: "failed to get path info: ",
		},
		{
			Name:           "successfully gets policy bundle for a policyPath of a FILE",
			PolicyPath:     "./testdata/test.rego",
			ExpectedOutput: map[string]string{"test.rego": "package test"},
		},
		{
			Name:           "successfully gets policy bundle for a policyPath of a DIRECTORY, also ignores subdirectories",
			PolicyPath:     "./testdata/test3/policies",
			ExpectedOutput: map[string]string{"a.rego": "package a", "b.rego": "package b"},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			documentBundle, err := getDocumentBundleFromPath(tc.PolicyPath)

			if tc.ExpectedErr == "" {
				assert.NilError(t, err)
				fmt.Println(tc.ExpectedOutput)
				assert.DeepEqual(t, tc.ExpectedOutput, documentBundle)
			} else {
				assert.ErrorContains(t, err, tc.ExpectedErr)
			}
		})
	}
}

func TestSetFileContentToMap(t *testing.T) {
	testcases := []struct {
		Name           string
		FilePath       string
		Key            string
		ContentMap     map[string]string
		ExpectedOutput map[string]string
		ExpectedErr    string
	}{
		{
			Name:        "fails on empty key",
			FilePath:    "./testdata/test.rego",
			ContentMap:  map[string]string{},
			ExpectedErr: "invalid key",
		},
		{
			Name:        "fails on uninitialized content map",
			FilePath:    "./testdata/test.rego",
			Key:         "test_key",
			ExpectedErr: "uninitialized contentMap",
		},
		{
			Name:        "fails on non-existing filePath",
			FilePath:    "./testdata/does_not_exist",
			Key:         "test_key",
			ContentMap:  map[string]string{},
			ExpectedErr: "failed to read file: open ./testdata/does_not_exist: ",
		},
		{
			Name:           "successfully sets file content to map",
			FilePath:       "./testdata/test.rego",
			Key:            "test.rego",
			ContentMap:     map[string]string{"existing_key": "existing_data"},
			ExpectedOutput: map[string]string{"existing_key": "existing_data", "test.rego": "package test"},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			err := setFileContentToMap(tc.FilePath, tc.Key, tc.ContentMap)

			if tc.ExpectedErr == "" {
				assert.NilError(t, err)
				assert.DeepEqual(t, tc.ExpectedOutput, tc.ContentMap)
			} else {
				assert.ErrorContains(t, err, tc.ExpectedErr)
			}
		})
	}
}
