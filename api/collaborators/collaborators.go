package collaborators

type CollaborationResult struct {
	VcsTye    string `json:"vcs_type"`
	OrgSlug   string `json:"slug"`
	OrgName   string `json:"name"`
	OrgId     string `json:"id"`
	AvatarUrl string `json:"avatar_url"`
}

type CollaboratorsClient interface {
	GetOrgCollaborations() ([]CollaborationResult, error)
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
