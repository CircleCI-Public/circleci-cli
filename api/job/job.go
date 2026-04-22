package job

// JobClient is the interface for fetching job details, test results, and logs.
type JobClient interface {
	GetTestResults(projectSlug string, jobNumber int) ([]TestResult, error)
	GetJobSteps(projectSlug string, jobNumber int) (*JobDetails, error)
	GetStepLog(projectSlug string, jobNumber int, taskIndex int, stepID int, logType string) (string, error)
}

// TestResult represents a single test result.
type TestResult struct {
	Name      string  `json:"name"`
	Classname string  `json:"classname"`
	Result    string  `json:"result"`
	Message   string  `json:"message"`
	Source    string  `json:"source"`
	RunTime   float64 `json:"run_time"`
}

// JobDetails holds detailed information about a job including its steps.
type JobDetails struct {
	BuildNum  int          `json:"build_num"`
	Status    string       `json:"status"`
	Steps     []JobStep    `json:"steps"`
	Workflows JobWorkflows `json:"workflows"`
}

// JobStep represents a step in a job.
type JobStep struct {
	Name    string      `json:"name"`
	Actions []JobAction `json:"actions"`
}

// JobAction represents an action (execution) within a step.
type JobAction struct {
	Index  int   `json:"index"`
	Step   int   `json:"step"`
	Failed *bool `json:"failed"`
}

// JobWorkflows holds workflow metadata associated with a job.
type JobWorkflows struct {
	JobName string `json:"job_name"`
}
