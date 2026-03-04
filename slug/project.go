package slug

import (
	"fmt"
	"strings"
)

type Project struct {
	VCS  string
	Org  string
	Repo string
}

func ParseProject(projectSlug string) (Project, error) {
	slug := strings.TrimSpace(projectSlug)
	slug = strings.Trim(slug, "/")

	parts := strings.Split(slug, "/")
	if len(parts) != 3 {
		return Project{}, fmt.Errorf("invalid project slug %q (expected <vcs>/<org>/<repo>)", projectSlug)
	}

	for i := range parts {
		if parts[i] == "" {
			return Project{}, fmt.Errorf("invalid project slug %q (empty segment)", projectSlug)
		}
	}

	return Project{
		VCS:  parts[0],
		Org:  parts[1],
		Repo: parts[2],
	}, nil
}

func (p Project) V1VCS() (string, error) {
	switch strings.ToLower(p.VCS) {
	case "gh", "github":
		return "github", nil
	case "bb", "bitbucket":
		return "bitbucket", nil
	default:
		return "", fmt.Errorf("unsupported vcs %q for v1.1 job logs", p.VCS)
	}
}
