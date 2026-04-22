package job_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/api/job"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"gotest.tools/v3/assert"
)

const (
	testProjectSlug = "gh/test-org/test-repo"
	testJobNumber   = 123
)

// getJobRestClient creates a client pointing all three API bases at the same test server.
func getJobRestClient(server *httptest.Server) (job.JobClient, error) {
	return job.NewJobRestClient(settings.Config{
		RestEndpoint: "api/v2",
		Host:         server.URL,
		HTTPClient:   &http.Client{},
		Token:        "token",
	})
}

func Test_jobRestClient_GetTestResults(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    []job.TestResult
		wantErr bool
	}{
		{
			name: "should return test results on success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/project/%s/%d/tests", testProjectSlug, testJobNumber))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{
					"items": [
						{
							"name": "TestFoo",
							"classname": "com.example.FooTest",
							"result": "failure",
							"message": "expected 3 got 4",
							"source": "junit",
							"run_time": 0.5
						}
					],
					"next_page_token": ""
				}`))
				assert.NilError(t, err)
			},
			want: []job.TestResult{
				{
					Name:      "TestFoo",
					Classname: "com.example.FooTest",
					Result:    "failure",
					Message:   "expected 3 got 4",
					Source:    "junit",
					RunTime:   0.5,
				},
			},
		},
		{
			name: "should paginate through all results",
			handler: func() http.HandlerFunc {
				callCount := 0
				return func(w http.ResponseWriter, r *http.Request) {
					callCount++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if callCount == 1 {
						_, err := w.Write([]byte(`{"items": [{"name": "TestA", "result": "success"}], "next_page_token": "page2"}`))
						assert.NilError(t, err)
					} else {
						_, err := w.Write([]byte(`{"items": [{"name": "TestB", "result": "failure"}], "next_page_token": ""}`))
						assert.NilError(t, err)
					}
				}
			}(),
			want: []job.TestResult{
				{Name: "TestA", Result: "success"},
				{Name: "TestB", Result: "failure"},
			},
		},
		{
			name: "should return error on API failure",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"message": "internal error"}`))
				assert.NilError(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client, err := getJobRestClient(server)
			assert.NilError(t, err)

			got, err := client.GetTestResults(testProjectSlug, testJobNumber)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func Test_jobRestClient_GetJobSteps(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    *job.JobDetails
		wantErr bool
	}{
		{
			name: "should return job details on success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v1.1/project/%s/%d", testProjectSlug, testJobNumber))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{
					"build_num": 123,
					"steps": [
						{
							"name": "Run tests",
							"actions": [
								{"index": 0, "step": 99, "failed": true}
							]
						}
					],
					"workflows": {"job_name": "test"}
				}`))
				assert.NilError(t, err)
			},
			want: func() *job.JobDetails {
				failed := true
				return &job.JobDetails{
					BuildNum: 123,
					Steps: []job.JobStep{
						{
							Name: "Run tests",
							Actions: []job.JobAction{
								{Index: 0, Step: 99, Failed: &failed},
							},
						},
					},
					Workflows: job.JobWorkflows{JobName: "test"},
				}
			}(),
		},
		{
			name: "should return error on API failure",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, err := w.Write([]byte(`{"message": "job not found"}`))
				assert.NilError(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client, err := getJobRestClient(server)
			assert.NilError(t, err)

			got, err := client.GetJobSteps(testProjectSlug, testJobNumber)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func Test_jobRestClient_GetStepLog(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
		wantErr bool
	}{
		{
			name: "should return raw log on success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/private/output/raw/%s/%d/output/0/99", testProjectSlug, testJobNumber))

				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("test log output"))
				assert.NilError(t, err)
			},
			want: "test log output",
		},
		{
			name: "should return error on API failure",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte("server error"))
				assert.NilError(t, err)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client, err := getJobRestClient(server)
			assert.NilError(t, err)

			got, err := client.GetStepLog(testProjectSlug, testJobNumber, 0, 99, "output")
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}
