package slug

import "testing"

func TestParseProject(t *testing.T) {
	t.Run("parses valid project slug", func(t *testing.T) {
		p, err := ParseProject("gh/GetJobber/lakehouse-event-collector")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.VCS != "gh" || p.Org != "GetJobber" || p.Repo != "lakehouse-event-collector" {
			t.Fatalf("unexpected project: %#v", p)
		}
	})

	t.Run("trims whitespace and slashes", func(t *testing.T) {
		p, err := ParseProject(" /gh/GetJobber/lakehouse-event-collector/ ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.VCS != "gh" || p.Org != "GetJobber" || p.Repo != "lakehouse-event-collector" {
			t.Fatalf("unexpected project: %#v", p)
		}
	})

	t.Run("rejects invalid slug", func(t *testing.T) {
		_, err := ParseProject("gh/GetJobber")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestProject_V1VCS(t *testing.T) {
	tests := []struct {
		name    string
		vcs     string
		want    string
		wantErr bool
	}{
		{name: "gh maps to github", vcs: "gh", want: "github"},
		{name: "github stays github", vcs: "github", want: "github"},
		{name: "bb maps to bitbucket", vcs: "bb", want: "bitbucket"},
		{name: "bitbucket stays bitbucket", vcs: "bitbucket", want: "bitbucket"},
		{name: "unknown errors", vcs: "circleci", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Project{VCS: tc.vcs}.V1VCS()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
