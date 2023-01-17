package git

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type VcsType string

const (
	GitHub    VcsType = "GITHUB"
	Bitbucket VcsType = "BITBUCKET"
)

type Remote struct {
	VcsType      VcsType
	Organization string
	Project      string
}

// Parse the output of `git remote` to infer what VCS provider is being used by
// in the current working directory. The assumption is that the 'origin' remote
// will be a Bitbucket or GitHub project. This matching is a best effort approach,
// and pull requests are welcome to make it more robust.
func InferProjectFromGitRemotes() (*Remote, error) {

	remoteUrl, err := getRemoteUrl("origin")

	if err != nil {
		return nil, err
	}

	return findRemote(remoteUrl)
}

func findRemote(url string) (*Remote, error) {
	vcsType, slug, err := findProviderAndSlug(url)
	if err != nil {
		return nil, err
	}

	matches := strings.Split(slug, "/")

	if len(matches) != 2 {
		return nil, fmt.Errorf("Splitting '%s' into organization and project failed", slug)
	}

	return &Remote{
		VcsType:      vcsType,
		Organization: matches[0],
		Project:      strings.TrimSuffix(matches[1], ".git"),
	}, nil
}

func findProviderAndSlug(url string) (VcsType, string, error) {

	var vcsParsers = map[VcsType][]*regexp.Regexp{
		GitHub: {
			regexp.MustCompile(`^(?:ssh\://)?git@github\.com[:/](.*)`),
			regexp.MustCompile(`https://(?:.*@)?github\.com/(.*)`),
		},
		Bitbucket: {
			regexp.MustCompile(`^(?:ssh\://)?git@bitbucket\.org[:/](.*)`),
			regexp.MustCompile(`https://(?:.*@)?bitbucket\.org/(.*)`),
		},
	}

	for provider, regexes := range vcsParsers {
		for _, regex := range regexes {
			if matches := regex.FindStringSubmatch(url); matches != nil {
				return provider, matches[1], nil
			}
		}
	}

	return "", "", fmt.Errorf("Unknown git remote: %s", url)
}

func getRemoteUrl(remoteName string) (string, error) {

	// Ensure that git is on the path
	if _, err := exec.LookPath("git"); err != nil {
		return "", errors.New("Could not find 'git' on the path; this command requires git to be installed.")
	}

	// Ensure that we are in a git repository
	if output, err := exec.Command("git", "status").CombinedOutput(); err != nil {
		if strings.Contains(string(output), "not a git repository") {
			return "", errors.New("This command must be run from inside a git repository")
		}
		// If `git status` fails for any other reason, let's optimisticly continue
		// execution and allow the call to `git remote` to fail.
	}

	out, err := exec.Command("git", "remote", "get-url", remoteName).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error finding the %s git remote: %s",
			remoteName,
			strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func commandOutputOrDefault(cmd *exec.Cmd, defaultValue string) string {
	output, err := cmd.CombinedOutput()

	if err != nil {
		return defaultValue
	}

	return strings.TrimSpace(string(output))
}

func Branch() string {
	// Git 2.22 (Q2 2019)added `git branch --show-current`, but using
	// `git rev-parse` works in all versions.
	return commandOutputOrDefault(
		exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"),
		"main")
}

func Revision() string {
	return commandOutputOrDefault(
		exec.Command("git", "rev-parse", "HEAD"),
		"0000000000000000000000000000000000000000")
}

func Tag() string {
	return commandOutputOrDefault(
		exec.Command("git", "tag", "--points-at", "HEAD"),
		"")
}
