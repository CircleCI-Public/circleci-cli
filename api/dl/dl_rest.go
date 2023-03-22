package dl

import (
	"fmt"
	"net/url"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

const defaultDlHost = "https://dl.circleci.com"

type dlRestClient struct {
	client *rest.Client
}

// NewDlRestClient returns a new dlRestClient instance initialized with the
// values from the config.
func NewDlRestClient(config settings.Config) (*dlRestClient, error) { //
	// We don't want the user to use this with Server as that's nor supported
	// at them moment.  In order to detect this we look if there's a config file
	// or cli option that sets "host" to anything different than the default
	if config.Host != "" && config.Host != "https://circleci.com" {
		// Only error if there's no custom DlHost set. Since the end user can't
		// a custom value set this in the config file, this has to have been
		// manually been set in the code, presumably by the test suite to allow
		// talking to a mock server, and we want to allow that.
		if config.DlHost == "" {
			return nil, &CloudOnlyErr{}
		}
	}

	// what's the base URL?
	unparsedURL := defaultDlHost
	if config.DlHost != "" {
		unparsedURL = config.DlHost
	}

	baseURL, err := url.Parse(unparsedURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse dl host URL '%s'", unparsedURL)
	}

	httpclient := config.HTTPClient
	httpclient.Timeout = 10 * time.Second

	// the dl endpoint is hardcoded to https://dl.circleci.com, since currently
	// this implementation always refers to the cloud dl service
	return &dlRestClient{
		client: rest.New(
			baseURL,
			config.Token,
			httpclient,
		),
	}, nil
}

func (c dlRestClient) PurgeDLC(projectid string) error {
	// this calls a private circleci endpoint.  We make no guarantees about
	// this still existing in the future.
	path := fmt.Sprintf("private/output/project/%s/dlc", projectid)
	req, err := c.client.NewRequest("DELETE", &url.URL{Path: path}, nil)
	if err != nil {
		return err
	}

	status, err := c.client.DoRequest(req, nil)

	// Futureproofing: If CircleCI ever removes the private backend endpoint
	// this call uses, by having the endpoint return a 410 status code CircleCI
	// can get everyone running an outdated client to display a helpful error
	// telling them to upgrade (presumably by this point a version without this
	// logic will have been released)
	if status == 410 {
		return &GoneErr{}
	}

	return err
}
