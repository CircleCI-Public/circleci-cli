package pipeline

type CreatePipelineInfo struct {
	Id                         string
	Name                       string
	CheckoutSourceRepoFullName string
	ConfigSourceRepoFullName   string
}

type PipelineDefinition struct {
	ConfigSourceId   string
	CheckoutSourceId string
}

type GetPipelineDefinitionOptions struct {
	ProjectID            string
	PipelineDefinitionID string
}

type PipelineRunCreatedResponse struct {
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	Number    int    `json:"number"`
	ID        string `json:"id"`
}

type PipelineRunMessageResponse struct {
	Message string `json:"message"`
}

type PipelineRunResponse struct {
	Created *PipelineRunCreatedResponse
	Message *PipelineRunMessageResponse
}

type Config struct {
	Branch  string `json:"branch,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Content string `json:"content,omitempty"`
}

type Checkout struct {
	Branch string `json:"branch,omitempty"`
	Tag    string `json:"tag,omitempty"`
}

type PipelineRunOptions struct {
	Project              string
	PipelineDefinitionID string
	Organization         string
	ConfigBranch         string
	ConfigTag            string
	CheckoutBranch       string
	CheckoutTag          string
	Parameters           map[string]interface{}
	ConfigFilePath       string
}

// PipelineClient is the interface to interact with pipeline and it's
// components.
type PipelineClient interface {
	CreatePipeline(projectID string, name string, description string, repoID string, configRepoID string, filePath string) (*CreatePipelineInfo, error)
	GetPipelineDefinition(options GetPipelineDefinitionOptions) (*PipelineDefinition, error)
	ListPipelineDefinitions(projectID string) ([]*PipelineDefinitionInfo, error)
	PipelineRun(options PipelineRunOptions) (*PipelineRunResponse, error)
}
