package collaborators

type CollaborationResult struct {
	VcsType   string `json:"vcs_type"`
	OrgSlug   string `json:"slug"`
	OrgName   string `json:"name"`
	OrgId     string `json:"id"`
	AvatarUrl string `json:"avatar_url"`
}

type CollaboratorsClient interface {
	GetCollaborationBySlug(slug string) (*CollaborationResult, error)
	GetOrgCollaborations() ([]CollaborationResult, error)
}
