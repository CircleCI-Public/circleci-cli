package process

import (
	"io/ioutil"
	"os"
	"testing"
)

func Test_MaybeIncludeFile(t *testing.T) {
	testCases := []struct {
		description string
		template    string
		expected    string
		files       map[string]string
		errExpected bool
	}{
		{
			description: "File gets replaced",
			template:    "<<include(example.txt)>>",
			expected:    "world",
			files: map[string]string{
				"example.txt": "world",
			},
			errExpected: false,
		},
		{
			description: "Partial line include",
			template:    "hello <<include(example-2.txt)>>",
			expected:    "hello world",
			files: map[string]string{
				"example-2.txt": "world",
			},
			errExpected: false,
		},
		{
			description: "Multiple includes",
			template:    "<<include(example-1.txt)>> <<include(example-2.txt)>>",
			expected:    "hello world",
			files: map[string]string{
				"example-1.txt": "hello",
				"example-2.txt": "world",
			},
			errExpected: false,
		},
		{
			description: "File does not exist",
			template:    "<<include(file-that-does-not-exist.txt)>>",
			files:       map[string]string{},
			errExpected: true,
		},
		{
			description: "Included files are escaped",
			template:    "<<include(example-1.txt)>> world",
			expected:    "\\<< hello world",
			files: map[string]string{
				"example-1.txt": "<< hello",
			},
			errExpected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "circleci-cli-test")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			for name, content := range tc.files {
				if err := ioutil.WriteFile(dir+"/"+name, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			orbDirectory := dir
			res, err := MaybeIncludeFile(tc.template, orbDirectory)
			if err != nil && !tc.errExpected {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tc.errExpected && res != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, res)
			}
		})
	}
}
