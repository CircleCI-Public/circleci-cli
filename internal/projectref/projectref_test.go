// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package projectref_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/projectref"
)

const (
	orgUUID  = "c1e89d5c-d2e5-4db2-b2d7-a35cf73160ad"
	projUUID = "3b524838-a95e-44eb-bc56-deb0af23ef19"

	// The base58-encoded versions of the above IDs
	orgB58  = "E6i3yYZeWZhcf8UNqcKfjN"
	projB58 = "13c8F7nusayivoSxC6GMsw"
)

// writeRaw writes body verbatim to .circleci/info.yml inside dir.
func writeRaw(t *testing.T, dir, body string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci"), 0o755))
	assert.NilError(t, os.WriteFile(projectref.Path(dir), []byte(body), 0o644))
}

func readRaw(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(projectref.Path(dir))
	assert.NilError(t, err)
	return string(b)
}

// rawDoc is the on-disk shape of info.yml, decoded without going through projectref so that
// tests can see exactly what Write emitted.
type rawDoc struct {
	Organization struct {
		ID   string `yaml:"id"`
		Name string `yaml:"name"`
	} `yaml:"organization"`
	Project struct {
		ID   string `yaml:"id"`
		Slug string `yaml:"slug"`
		Name string `yaml:"name"`
	} `yaml:"project"`
}

func readDoc(t *testing.T, dir string) rawDoc {
	t.Helper()
	var doc rawDoc
	assert.NilError(t, yaml.Unmarshal([]byte(readRaw(t, dir)), &doc))
	return doc
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

type infoCase struct {
	name string
	info projectref.Info
}

// validInfos are records that projectref.Write must accept and projectref.Read must return unchanged.
var validInfos = []infoCase{
	{
		name: "vcs slug with ids and names",
		info: projectref.Info{
			Organization: projectref.Organization{ID: orgUUID, Name: "myorg"},
			Project:      projectref.Project{ID: projUUID, Slug: "gh/myorg/myrepo", Name: "myrepo"},
		},
	},
	{
		name: "circleci-native base58 slug with ids",
		info: projectref.Info{
			Organization: projectref.Organization{ID: orgUUID},
			Project:      projectref.Project{ID: projUUID, Slug: "circleci/" + orgB58 + "/" + projB58},
		},
	},
	{
		name: "circleci-native uuid slug with ids",
		info: projectref.Info{
			Organization: projectref.Organization{ID: orgUUID},
			Project:      projectref.Project{ID: projUUID, Slug: "circleci/" + orgUUID + "/" + projUUID},
		},
	},
	{
		name: "slug only",
		info: projectref.Info{Project: projectref.Project{Slug: "bb/myorg/myrepo"}},
	},
	{
		name: "unicode names",
		info: projectref.Info{
			Organization: projectref.Organization{ID: orgUUID, Name: "Ünïcødé Örg 🚀"},
			Project:      projectref.Project{ID: projUUID, Slug: "circleci/" + orgB58 + "/" + projB58, Name: "日本語"},
		},
	},
}

// longPrefixSlugs maps long-form VCS slugs, which projectref.Read and projectref.Write accept,
// to the short form they normalise to.
var longPrefixSlugs = map[string]string{
	"github/myorg/myrepo":    "gh/myorg/myrepo",
	"bitbucket/myorg/myrepo": "bb/myorg/myrepo",
}

// invalidSlugs are project slugs that projectref.Read must reject and projectref.Write must refuse.
var invalidSlugs = map[string]string{
	"empty":                     "",
	"whitespace only":           "   ",
	"two segments":              "gh/org",
	"four segments":             "gh/org/repo/extra",
	"empty middle segment":      "gh//repo",
	"leading slash":             "/org/repo",
	"trailing slash":            "gh/org/repo/",
	"leading whitespace":        " gh/org/repo",
	"trailing newline":          "gh/org/repo\n",
	"inner space":               "gh/org/re po",
	"dot segments":              "gh/../..",
	"gitlab prefix":             "gitlab/org/repo",
	"unknown prefix":            "xx/org/repo",
	"uppercase gh prefix":       "GH/org/repo",
	"uppercase circleci":        "Circleci/" + orgB58 + "/" + projB58,
	"circleci two segments":     "circleci/" + orgB58,
	"circleci base58 then uuid": "circleci/" + orgB58 + "/" + projUUID,
	"circleci uuid then base58": "circleci/" + orgUUID + "/" + projB58,
}

// invalidInfos are records that projectref.Read must reject when found on disk and that
// projectref.Write must refuse to produce.
var invalidInfos = func() []infoCase {
	fields := []infoCase{
		{name: "org id not a uuid", info: projectref.Info{Organization: projectref.Organization{ID: "not-a-uuid"}, Project: projectref.Project{Slug: "gh/o/r"}}},
		{name: "org id path traversal", info: projectref.Info{Organization: projectref.Organization{ID: "../.."}, Project: projectref.Project{Slug: "gh/o/r"}}},
		{name: "org id with slash", info: projectref.Info{Organization: projectref.Organization{ID: "a/b"}, Project: projectref.Project{Slug: "gh/o/r"}}},
		{name: "org id padded", info: projectref.Info{Organization: projectref.Organization{ID: " " + orgUUID}, Project: projectref.Project{Slug: "gh/o/r"}}},
		{name: "org id base58", info: projectref.Info{Organization: projectref.Organization{ID: orgB58}, Project: projectref.Project{Slug: "gh/o/r"}}},
		{name: "project id not a uuid", info: projectref.Info{Project: projectref.Project{ID: "not-a-uuid", Slug: "gh/o/r"}}},
		{name: "project id path traversal", info: projectref.Info{Project: projectref.Project{ID: "../..", Slug: "gh/o/r"}}},
		{name: "project id with slash", info: projectref.Info{Project: projectref.Project{ID: "a/b", Slug: "gh/o/r"}}},
		{name: "project id padded", info: projectref.Info{Project: projectref.Project{ID: projUUID + " ", Slug: "gh/o/r"}}},
		{name: "project id base58", info: projectref.Info{Project: projectref.Project{ID: projB58, Slug: "gh/o/r"}}},
		{name: "org name whitespace", info: projectref.Info{Organization: projectref.Organization{Name: "   "}, Project: projectref.Project{Slug: "gh/o/r"}}},
		{name: "org name tab", info: projectref.Info{Organization: projectref.Organization{Name: "\t"}, Project: projectref.Project{Slug: "gh/o/r"}}},
		{name: "project name whitespace", info: projectref.Info{Project: projectref.Project{Slug: "gh/o/r", Name: "   "}}},
		{name: "project name newline", info: projectref.Info{Project: projectref.Project{Slug: "gh/o/r", Name: "\n"}}},
	}
	out := make([]infoCase, 0, len(fields)+len(invalidSlugs))
	out = append(out, fields...)
	for name, slug := range invalidSlugs {
		out = append(out, infoCase{name: "slug " + name, info: projectref.Info{Project: projectref.Project{Slug: slug}}})
	}
	return out
}()

func TestRead_Valid(t *testing.T) {
	t.Run("round trip of written records", func(t *testing.T) {
		for _, tc := range validInfos {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				assert.NilError(t, projectref.Write(dir, &tc.info))
				got, err := projectref.Read(dir)
				assert.NilError(t, err)
				assert.Check(t, cmp.DeepEqual(*got, tc.info))
			})
		}
	})

	raw := "organization:\n  id: " + orgUUID + "\nproject:\n  id: " + projUUID + "\n  slug: gh/myorg/myrepo\n"
	want := projectref.Info{
		Organization: projectref.Organization{ID: orgUUID},
		Project:      projectref.Project{ID: projUUID, Slug: "gh/myorg/myrepo"},
	}
	for name, body := range map[string]string{
		"plain":        raw,
		"utf-8 bom":    "\ufeff" + raw,
		"crlf endings": strings.ReplaceAll(raw, "\n", "\r\n"),
		"comments":     "# linked by circleci\n" + raw,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeRaw(t, dir, body)
			got, err := projectref.Read(dir)
			assert.NilError(t, err)
			assert.Check(t, cmp.DeepEqual(*got, want))
		})
	}
}

func TestRead_NotFound(t *testing.T) {
	t.Run("no .circleci directory", func(t *testing.T) {
		_, err := projectref.Read(t.TempDir())
		assert.Check(t, cmp.ErrorIs(err, projectref.ErrNotFound))
	})

	t.Run("no info.yml", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, os.Mkdir(filepath.Join(dir, ".circleci"), 0o755))
		_, err := projectref.Read(dir)
		assert.Check(t, cmp.ErrorIs(err, projectref.ErrNotFound))
	})

	t.Run("info.yml is a directory", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, os.MkdirAll(projectref.Path(dir), 0o755))
		_, err := projectref.Read(dir)
		assert.Check(t, err != nil)
		assert.Check(t, !strings.Contains(errString(err), projectref.ErrNotFound.Error()), "a directory is not a missing file: %v", err)
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestRead_Malformed(t *testing.T) {
	tests := map[string]string{
		"empty file":         "",
		"null document":      "null\n",
		"top-level list":     "- a\n- b\n",
		"top-level scalar":   "gh/myorg/myrepo\n",
		"project is scalar":  "project: gh/myorg/myrepo\n",
		"slug missing":       "project:\n  id: " + projUUID + "\n",
		"slug is list":       "project:\n  slug: [a]\n",
		"slug is null":       "project:\n  slug: ~\n",
		"slug is mapping":    "project:\n  slug:\n    a: b\n",
		"duplicate slug key": "project:\n  slug: gh/a/b\n  slug: gh/c/d\n",
		"tab indentation":    "project:\n\tslug: gh/a/b\n",
		"binary garbage":     "\x00\x01\x02\xff",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeRaw(t, dir, body)
			got, err := projectref.Read(dir)
			assert.Check(t, err != nil, "Read accepted malformed file, returned %+v", got)
		})
	}
}

func TestRead_SlugShape(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tests := map[string]string{
			"github":                  "gh/myorg/myrepo",
			"bitbucket":               "bb/myorg/myrepo",
			"punctuation in names":    "gh/my-org/my.repo_name",
			"circleci base58":         "circleci/" + orgB58 + "/" + projB58,
			"circleci uuid":           "circleci/" + orgUUID + "/" + projUUID,
			"circleci uppercase uuid": "circleci/" + strings.ToUpper(orgUUID) + "/" + strings.ToUpper(projUUID),
		}
		for name, slug := range tests {
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				writeRaw(t, dir, "project:\n  slug: "+slug+"\n")
				_, err := projectref.Read(dir)
				assert.Check(t, err, "Read rejected slug %q", slug)
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		tests := map[string]string{
			"integer": "123",
			"boolean": "true",
		}
		for name, slug := range invalidSlugs {
			tests[name] = `"` + strings.ReplaceAll(slug, "\n", `\n`) + `"`
		}
		for name, slug := range tests {
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				writeRaw(t, dir, "project:\n  slug: "+slug+"\n")
				got, err := projectref.Read(dir)
				assert.Check(t, err != nil, "Read accepted slug %s, returned %q", slug, slugOf(got))
			})
		}
	})

	t.Run("normalises long vcs prefixes", func(t *testing.T) {
		for slug, want := range longPrefixSlugs {
			t.Run(slug, func(t *testing.T) {
				dir := t.TempDir()
				writeRaw(t, dir, "project:\n  slug: "+slug+"\n")
				got, err := projectref.Read(dir)
				assert.NilError(t, err)
				assert.Check(t, cmp.Equal(got.Project.Slug, want))
			})
		}
	})
}

func slugOf(i *projectref.Info) string {
	if i == nil {
		return ""
	}
	return i.Project.Slug
}

func TestRead_IDs(t *testing.T) {
	valid := map[string]string{
		"explicit empty": `""`,
		"canonical":      orgUUID,
		"uppercase":      strings.ToUpper(orgUUID),
		"braces":         `"{` + orgUUID + `}"`,
		"urn":            "urn:uuid:" + orgUUID,
		"no hyphens":     strings.ReplaceAll(orgUUID, "-", ""),
	}
	invalid := map[string]string{
		"not a uuid":     "not-a-uuid",
		"path traversal": `"../.."`,
		"slash":          "a/b",
		"padded":         `" ` + orgUUID + `"`,
		"integer":        "123",
		"boolean":        "true",
		"base58":         orgB58,
	}

	for _, field := range []string{"organization", "project"} {
		t.Run(field, func(t *testing.T) {
			t.Run("absent is valid", func(t *testing.T) {
				dir := t.TempDir()
				writeRaw(t, dir, "project:\n  slug: gh/o/r\n")
				_, err := projectref.Read(dir)
				assert.Check(t, err)
			})

			for name, id := range valid {
				t.Run(name+" is valid", func(t *testing.T) {
					dir := t.TempDir()
					writeRaw(t, dir, idDoc(field, id))
					_, err := projectref.Read(dir)
					assert.Check(t, err, "Read rejected %s.id %s", field, id)
				})
			}

			for name, id := range invalid {
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					writeRaw(t, dir, idDoc(field, id))
					got, err := projectref.Read(dir)
					assert.Check(t, err != nil, "Read accepted %s.id %s, returned %+v", field, id, got)
				})
			}
		})
	}
}

func idDoc(field, id string) string {
	if field == "organization" {
		return "organization:\n  id: " + id + "\nproject:\n  slug: gh/o/r\n"
	}
	return "project:\n  id: " + id + "\n  slug: gh/o/r\n"
}

func TestRead_Names(t *testing.T) {
	invalid := map[string]string{
		"whitespace only": `"   "`,
		"tab only":        `"\t"`,
	}

	for _, field := range []string{"organization", "project"} {
		t.Run(field, func(t *testing.T) {
			t.Run("absent is valid", func(t *testing.T) {
				dir := t.TempDir()
				writeRaw(t, dir, "project:\n  slug: gh/o/r\n")
				_, err := projectref.Read(dir)
				assert.Check(t, err)
			})

			t.Run("unicode is valid", func(t *testing.T) {
				dir := t.TempDir()
				writeRaw(t, dir, nameDoc(field, "Ünïcødé 🚀"))
				_, err := projectref.Read(dir)
				assert.Check(t, err)
			})

			for name, value := range invalid {
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					writeRaw(t, dir, nameDoc(field, value))
					got, err := projectref.Read(dir)
					assert.Check(t, err != nil, "Read accepted %s.name %s, returned %+v", field, value, got)
				})
			}
		})
	}
}

// nameDoc uses a CircleCI-native slug: for gh/ and bb/ slugs the names always match the slug's
// segments, so free-form names only occur on native projects.
func nameDoc(field, name string) string {
	slug := "circleci/" + orgB58 + "/" + projB58
	if field == "organization" {
		return "organization:\n  name: " + name + "\nproject:\n  slug: " + slug + "\n"
	}
	return "project:\n  name: " + name + "\n  slug: " + slug + "\n"
}

func TestRead_UnknownKeys(t *testing.T) {
	tests := map[string]struct {
		body string
		key  string
	}{
		"misspelt project":      {body: "project:\n  slug: gh/o/r\nprojet:\n  id: " + projUUID + "\n", key: "projet"},
		"british organisation":  {body: "organisation:\n  id: " + orgUUID + "\nproject:\n  slug: gh/o/r\n", key: "organisation"},
		"flat project_id":       {body: "project_id: " + projUUID + "\nproject:\n  slug: gh/o/r\n", key: "project_id"},
		"flat organization_id":  {body: "organization_id: " + orgUUID + "\nproject:\n  slug: gh/o/r\n", key: "organization_id"},
		"misspelt nested key":   {body: "project:\n  slug: gh/o/r\n  idd: " + projUUID + "\n", key: "idd"},
		"unknown top-level key": {body: "project:\n  slug: gh/o/r\nbranch: main\n", key: "branch"},
	}
	want := projectref.Info{Project: projectref.Project{Slug: "gh/o/r"}}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeRaw(t, dir, tc.body)

			var got *projectref.Info
			assert.Assert(t, t.Run("ignored on read", func(t *testing.T) {
				var err error
				got, err = projectref.Read(dir)
				assert.NilError(t, err)
				assert.Check(t, cmp.DeepEqual(*got, want))
			}))

			t.Run("dropped on write", func(t *testing.T) {
				assert.NilError(t, projectref.Write(dir, got))
				raw := readRaw(t, dir)
				assert.Check(t, !strings.Contains(raw, tc.key), "unknown key %q survived a rewrite: %q", tc.key, raw)
			})
		})
	}
}

func TestEffectiveSlug(t *testing.T) {
	nativeSlug := "circleci/" + orgB58 + "/" + projB58

	t.Run("nil receiver", func(t *testing.T) {
		var i *projectref.Info
		assert.Check(t, cmp.Equal(i.EffectiveSlug(), ""))
	})

	tests := []struct {
		name string
		info projectref.Info
		want string
	}{
		{
			name: "vcs slug keeps stored slug",
			info: projectref.Info{Organization: projectref.Organization{ID: orgUUID}, Project: projectref.Project{ID: projUUID, Slug: "gh/o/r"}},
			want: "gh/o/r",
		},
		{
			name: "native slug with both ids uses ids",
			info: projectref.Info{Organization: projectref.Organization{ID: orgUUID}, Project: projectref.Project{ID: projUUID, Slug: nativeSlug}},
			want: "circleci/" + orgUUID + "/" + projUUID,
		},
		{
			name: "native slug missing org id keeps stored slug",
			info: projectref.Info{Project: projectref.Project{ID: projUUID, Slug: nativeSlug}},
			want: nativeSlug,
		},
		{
			name: "native slug missing project id keeps stored slug",
			info: projectref.Info{Organization: projectref.Organization{ID: orgUUID}, Project: projectref.Project{Slug: nativeSlug}},
			want: nativeSlug,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(tc.info.EffectiveSlug(), tc.want))
		})
	}

	t.Run("always three non-empty segments", func(t *testing.T) {
		for _, id := range []string{"a/b", "../..", "", " ", "x/"} {
			info := projectref.Info{
				Organization: projectref.Organization{
					ID: id,
				},
				Project: projectref.Project{
					ID:   id,
					Slug: nativeSlug,
				},
			}
			got := info.EffectiveSlug()
			parts := strings.Split(got, "/")
			ok := len(parts) == 3
			for _, p := range parts {
				ok = ok && strings.TrimSpace(p) != "" && p != ".."
			}
			assert.Check(t, ok, "ids %q produced slug %q", id, got)
		}
	})
}

func TestWrite(t *testing.T) {
	t.Run("creates directory and file with expected modes", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX permissions")
		}
		dir := t.TempDir()
		assert.NilError(t, projectref.Write(dir, &validInfos[0].info))

		dirInfo, err := os.Stat(filepath.Join(dir, ".circleci"))
		assert.NilError(t, err)
		assert.Check(t, dirInfo.IsDir())
		fileInfo, err := os.Stat(projectref.Path(dir))
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(fileInfo.Mode().Perm(), os.FileMode(0o644)))
	})

	t.Run("nil info", func(t *testing.T) {
		dir := t.TempDir()
		err := projectref.Write(dir, nil)
		assert.Check(t, err != nil, "Write(nil) succeeded")
		assert.Check(t, !fileExists(projectref.Path(dir)), "Write(nil) left a file behind")
	})

	t.Run("refuses records projectref.Read would reject", func(t *testing.T) {
		for _, tc := range invalidInfos {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				err := projectref.Write(dir, &tc.info)
				assert.Check(t, err != nil, "Write accepted %+v", tc.info)
				assert.Check(t, !fileExists(projectref.Path(dir)), "Write left a file behind")
			})
		}
	})

	// Write may either reject a non-canonical UUID or normalise it; it must never persist one.
	t.Run("writes only canonical ids", func(t *testing.T) {
		forms := map[string]func(string) string{
			"uppercase":  strings.ToUpper,
			"braces":     func(s string) string { return "{" + s + "}" },
			"urn":        func(s string) string { return "urn:uuid:" + s },
			"no hyphens": func(s string) string { return strings.ReplaceAll(s, "-", "") },
		}
		for name, form := range forms {
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				info := projectref.Info{
					Organization: projectref.Organization{ID: form(orgUUID)},
					Project:      projectref.Project{ID: form(projUUID), Slug: "gh/o/r"},
				}
				if err := projectref.Write(dir, &info); err != nil {
					assert.Check(t, !fileExists(projectref.Path(dir)), "Write left a file behind")
					return
				}
				doc := readDoc(t, dir)
				assert.Check(t, cmp.Equal(doc.Organization.ID, orgUUID))
				assert.Check(t, cmp.Equal(doc.Project.ID, projUUID))
			})
		}
	})

	t.Run("writes only canonical uuid slugs", func(t *testing.T) {
		dir := t.TempDir()
		info := projectref.Info{
			Project: projectref.Project{Slug: "circleci/" + strings.ToUpper(orgUUID) + "/" + strings.ToUpper(projUUID)},
		}
		if err := projectref.Write(dir, &info); err != nil {
			assert.Check(t, !fileExists(projectref.Path(dir)), "Write left a file behind")
			return
		}
		assert.Check(t, cmp.Equal(readDoc(t, dir).Project.Slug, "circleci/"+orgUUID+"/"+projUUID))
	})

	t.Run("writes long vcs prefixes in short form", func(t *testing.T) {
		for slug, want := range longPrefixSlugs {
			t.Run(slug, func(t *testing.T) {
				dir := t.TempDir()
				assert.NilError(t, projectref.Write(dir, &projectref.Info{Project: projectref.Project{Slug: slug}}))
				assert.Check(t, cmp.Equal(readDoc(t, dir).Project.Slug, want))
			})
		}
	})

	t.Run("failed write keeps existing file", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, projectref.Write(dir, &validInfos[0].info))
		before := readRaw(t, dir)

		err := projectref.Write(dir, &projectref.Info{Project: projectref.Project{Slug: ""}})
		assert.Check(t, err != nil, "Write accepted an empty slug")
		assert.Check(t, cmp.Equal(readRaw(t, dir), before))
	})

	t.Run("leaves no temporary files", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, projectref.Write(dir, &validInfos[0].info))
		assert.NilError(t, projectref.Write(dir, &validInfos[1].info))
		entries, err := os.ReadDir(filepath.Join(dir, ".circleci"))
		assert.NilError(t, err)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.Check(t, cmp.DeepEqual(names, []string{"info.yml"}))
	})

	t.Run(".circleci is a regular file", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, os.WriteFile(filepath.Join(dir, ".circleci"), nil, 0o644))
		assert.Check(t, projectref.Write(dir, &validInfos[0].info) != nil)
	})
}

func TestWriteReadRoundTrip(t *testing.T) {
	for _, tc := range validInfos {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			assert.NilError(t, projectref.Write(dir, &tc.info))
			got, err := projectref.Read(dir)
			assert.NilError(t, err)
			assert.Check(t, cmp.DeepEqual(*got, tc.info))
		})
	}

	t.Run("omits empty optional fields", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, projectref.Write(dir, &projectref.Info{Project: projectref.Project{Slug: "gh/o/r"}}))
		raw := readRaw(t, dir)
		assert.Check(t, cmp.Contains(raw, "slug: gh/o/r"))
		for _, key := range []string{"id:", "name:"} {
			assert.Check(t, !strings.Contains(raw, key), "empty %s written: %q", key, raw)
		}
	})

	t.Run("drops explicitly empty ids read from disk", func(t *testing.T) {
		dir := t.TempDir()
		writeRaw(t, dir, "organization:\n  id: \"\"\nproject:\n  id: \"\"\n  slug: gh/o/r\n")
		got, err := projectref.Read(dir)
		assert.NilError(t, err)
		assert.NilError(t, projectref.Write(dir, got))
		raw := readRaw(t, dir)
		assert.Check(t, !strings.Contains(raw, "id:"), "empty id written: %q", raw)
	})
}
