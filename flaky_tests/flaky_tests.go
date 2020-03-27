package flaky_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

// pid 078d0d3b-a3af-4daa-9b4c-aad4a2a838dd

type ArrayResponse struct {
	Items []struct {
		Id        string
		JobNumber int `json:"job_number"`
		Name      string
		Status    string
	}
	NextPageToken string `json:"next_page_token"`
}

type Test struct {
	Message   string
	File      string
	Result    string
	Name      string
	Classname string
}

func DoIt(ctx context.Context, token, projectSlug, branch string) (string, error) {

	pipelines, err := getPipelines(ctx, token, projectSlug, branch)

	failures := []Test{}

	if err != nil {
		return "", err
	}

	for _, pipeline := range pipelines.Items {
		workflows, err := getWorkflows(ctx, token, pipeline.Id)

		if err != nil {
			return "", err
		}

		if len(workflows.Items) < 2 {
			continue
		}

		for _, workflow := range workflows.Items {

			if workflow.Status != "failed" {
				continue
			}

			jobs, err := getJobs(ctx, token, workflow.Id)

			if err != nil {
				return "", err
			}

			for _, job := range jobs.Items {

				if job.JobNumber == 0 {
					continue
				}

				if job.Status != "failed" {
					continue
				}

				// fmt.Printf("job: %d - %s\n", job.JobNumber, job.Name)

				results, err := getTestResults(ctx, token, projectSlug, job.JobNumber)

				if err != nil {
					return "", err
				}

				for _, test := range results.Items {

					if test.Result != "success" {
						failures = append(failures, test)
					}

				}
			}
		}
	}

	fmt.Println("")
	for _, test := range failures {
		fmt.Printf("%s %s %s: %s\n", test.File, test.Classname, test.Name, test.Result)
	}

	return "", nil
}

type TestResults struct {
	Items         []Test
	NextPageToken string `json:"next_page_token"`
}

func getTestResults(ctx context.Context, token, projectSlug string, jobNumber int) (*TestResults, error) {
	var resp TestResults
	url := fmt.Sprintf("https://circleci.com/api/v2/project/%s/%d/tests", projectSlug, jobNumber)
	err := getThings(ctx, token, url, &resp)
	return &resp, err
}

func getPipelines(ctx context.Context, token, projectSlug, branch string) (*ArrayResponse, error) {

	//page-token

	result, err := listThings(ctx, token, fmt.Sprintf("https://circleci.com/api/v2/project/%s/pipeline?&branch=%s", projectSlug, branch))

	if err != nil {
		return nil, err
	}

	pageToken := result.NextPageToken

	for i := 0; i < 5; i++ {

		things, err := listThings(ctx, token, fmt.Sprintf("https://circleci.com/api/v2/project/%s/pipeline?page-token=%s", projectSlug, pageToken))

		pageToken = things.NextPageToken

		if err != nil {
			return nil, errors.Wrap(err, "getting a page of pipelines failed")
		}

		result.Items = append(result.Items, things.Items...)

	}
	return result, nil
}

func getWorkflows(ctx context.Context, token string, pipelineId string) (*ArrayResponse, error) {
	return listThings(ctx, token, fmt.Sprintf("https://circleci.com/api/v2/pipeline/%s/workflow", pipelineId))
}

func getJobs(ctx context.Context, token string, workflowId string) (*ArrayResponse, error) {
	return listThings(ctx, token, fmt.Sprintf("https://circleci.com/api/v2/workflow/%s/job", workflowId))
}

func listThings(ctx context.Context, token, url string) (*ArrayResponse, error) {
	var resp ArrayResponse
	err := getThings(ctx, token, url, &resp)
	return &resp, err
}

func getThings(ctx context.Context, token, url string, v interface{}) error {

	fmt.Print(".")

	headers := map[string][]string{
		"Accept":       []string{"application/json"},
		"Circle-Token": []string{token},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)

	if err != nil {
		return err
	}
	req.Header = headers

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return err

	}

	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(body))
		return fmt.Errorf("status was %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
