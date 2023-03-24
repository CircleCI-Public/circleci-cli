package config

import (
	"net/url"
)

var (
	CollaborationsPath = "me/collaborations"
)

type CollaborationResult struct {
	VcsTye    string `json:"vcs_type"`
	OrgSlug   string `json:"slug"`
	OrgName   string `json:"name"`
	OrgId     string `json:"id"`
	AvatarUrl string `json:"avatar_url"`
}

// GetOrgCollaborations - fetches all the collaborations for a given user.
func (c *ConfigCompiler) GetOrgCollaborations() ([]CollaborationResult, error) {
	req, err := c.collaboratorRestClient.NewRequest("GET", &url.URL{Path: CollaborationsPath}, nil)
	if err != nil {
		return nil, err
	}

	var resp []CollaborationResult
	_, err = c.collaboratorRestClient.DoRequest(req, &resp)
	return resp, err
}

// GetOrgIdFromSlug - converts a slug into an orgID.
func GetOrgIdFromSlug(slug string, collaborations []CollaborationResult) string {
	for _, v := range collaborations {
		if v.OrgSlug == slug {
			return v.OrgId
		}
	}
	return ""
}
