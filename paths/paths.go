package paths

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/git"
)

func ProjectUrl(remote *git.Remote) string {
	return fmt.Sprintf("https://app.circleci.com/pipelines/%s/%s/%s",
		url.PathEscape(strings.ToLower(string(remote.VcsType))),
		url.PathEscape(remote.Organization),
		url.PathEscape(remote.Project))
}
