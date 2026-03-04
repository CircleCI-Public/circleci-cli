package workflow

type Job struct {
	CanceledBy        string                 `json:"canceled_by"`
	Dependencies      []string               `json:"dependencies"`
	JobNumber         int                    `json:"job_number"`
	ID                string                 `json:"id"`
	StartedAt         string                 `json:"started_at"`
	Name              string                 `json:"name"`
	ApprovedBy        string                 `json:"approved_by"`
	ProjectSlug       string                 `json:"project_slug"`
	Status            string                 `json:"status"`
	Type              string                 `json:"type"`
	Requires          map[string]interface{} `json:"requires"`
	StoppedAt         string                 `json:"stopped_at"`
	ApprovalRequestID string                 `json:"approval_request_id"`
}

type JobsResponse struct {
	NextPageToken string `json:"next_page_token"`
	Items         []Job  `json:"items"`
}

type WorkflowClient interface {
	ListWorkflowJobs(workflowID string) ([]Job, error)
}
