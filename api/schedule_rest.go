package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/pkg/errors"
)

// Communicates with the CircleCI REST API to ask questions about
// schedules. It satisfies api.ScheduleInterface.
type ScheduleRestClient struct {
	token  string
	server string
	client *http.Client
}

type listSchedulesResponse struct {
	Items         []Schedule
	NextPageToken *string `json:"next_page_token"`
	client        *ScheduleRestClient
	params        *listSchedulesParams
}

type listSchedulesParams struct {
	PageToken *string
}

// Creates a new schedule in the supplied project.
func (c *ScheduleRestClient) CreateSchedule(vcs, org, project, name, description string,
	useSchedulingSystem bool, timetable Timetable, parameters map[string]string) (*Schedule, error) {

	req, err := c.newCreateScheduleRequest(vcs, org, project, name, description, useSchedulingSystem, timetable, parameters)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 201 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		return nil, errors.New(*dest.Message)
	}

	var schedule Schedule
	if err := json.Unmarshal(bodyBytes, &schedule); err != nil {
		return nil, err
	}

	return &schedule, nil
}

// Updates an existing schedule.
func (c *ScheduleRestClient) UpdateSchedule(scheduleID, name, description string,
	useSchedulingSystem bool, timetable Timetable, parameters map[string]string) (*Schedule, error) {

	req, err := c.newUpdateScheduleRequest(scheduleID, name, description, useSchedulingSystem, timetable, parameters)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		return nil, errors.New(*dest.Message)
	}

	var schedule Schedule
	if err := json.Unmarshal(bodyBytes, &schedule); err != nil {
		return nil, err
	}

	return &schedule, nil
}

// Deletes the schedule with the given ID.
func (c *ScheduleRestClient) DeleteSchedule(scheduleID string) error {
	req, err := c.newDeleteScheduleRequest(scheduleID)

	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return err
		}
		return errors.New(*dest.Message)
	}
	return nil
}

// Returns all of the schedules for a given project. Note that
// pagination is not currently supported - we get all pages of
// schedules and return them all.
func (c *ScheduleRestClient) Schedules(vcs, org, project string) (*[]Schedule, error) {
	schedules, err := c.listAllSchedules(vcs, org, project, &listSchedulesParams{})
	return &schedules, err
}

// Returns the schedule with the given ID.
func (c *ScheduleRestClient) ScheduleByID(scheduleID string) (*Schedule, error) {
	req, err := c.newGetScheduleRequest(scheduleID)

	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err
		}
		return nil, errors.New(*dest.Message)
	}

	schedule := Schedule{}
	if err := json.Unmarshal(bodyBytes, &schedule); err != nil {
		return nil, err
	}

	return &schedule, err
}

// Finds a single schedule by its name and returns it.
func (c *ScheduleRestClient) ScheduleByName(vcs, org, project, name string) (*Schedule, error) {
	params := &listSchedulesParams{}
	for {
		resp, err := c.listSchedules(vcs, org, project, params)
		if err != nil {
			return nil, err
		}
		for _, schedule := range resp.Items {
			if schedule.Name == name {
				return &schedule, nil
			}
		}
		if resp.NextPageToken == nil {
			return nil, nil
		}
		params.PageToken = resp.NextPageToken
	}
}

// Fetches all pages of the schedule list API and returns a single
// list with all the schedules.
func (c *ScheduleRestClient) listAllSchedules(vcs, org, project string, params *listSchedulesParams) (schedules []Schedule, err error) {
	var resp *listSchedulesResponse
	for {
		resp, err = c.listSchedules(vcs, org, project, params)
		if err != nil {
			return nil, err
		}

		schedules = append(schedules, resp.Items...)

		if resp.NextPageToken == nil {
			break
		}

		params.PageToken = resp.NextPageToken
	}
	return schedules, nil
}

// Fetches and returns one page of schedules.
func (c *ScheduleRestClient) listSchedules(vcs, org, project string, params *listSchedulesParams) (*listSchedulesResponse, error) {
	req, err := c.newListSchedulesRequest(vcs, org, project, params)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		var dest errorResponse
		if err := json.Unmarshal(bodyBytes, &dest); err != nil {
			return nil, err

		}
		return nil, errors.New(*dest.Message)

	}

	dest := listSchedulesResponse{
		client: c,
		params: params,
	}
	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}
	return &dest, nil
}

// Builds a request to fetch a schedule by ID.
func (c *ScheduleRestClient) newGetScheduleRequest(scheduleID string) (*http.Request, error) {
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}

	queryURL, err = queryURL.Parse(fmt.Sprintf("schedule/%s", scheduleID))
	if err != nil {
		return nil, err
	}

	return c.newHTTPRequest("GET", queryURL.String(), nil)
}

// Builds a request to create a new schedule.
func (c *ScheduleRestClient) newCreateScheduleRequest(vcs, org, project, name, description string,
	useSchedulingSystem bool, timetable Timetable, parameters map[string]string) (*http.Request, error) {

	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("project/%s/%s/%s/schedule", vcs, org, project))
	if err != nil {
		return nil, err
	}

	actor := "current"
	if useSchedulingSystem {
		actor = "system"
	}

	var bodyReader io.Reader

	var body = struct {
		Name             string            `json:"name"`
		Description      string            `json:"description,omitempty"`
		AttributionActor string            `json:"attribution-actor"`
		Parameters       map[string]string `json:"parameters"`
		Timetable        Timetable         `json:"timetable"`
	}{
		Name:             name,
		Description:      description,
		AttributionActor: actor,
		Parameters:       parameters,
		Timetable:        timetable,
	}
	buf, err := json.Marshal(body)

	if err != nil {
		return nil, err
	}

	bodyReader = bytes.NewReader(buf)

	return c.newHTTPRequest("POST", queryURL.String(), bodyReader)
}

// Builds a request to update an existing schedule.
func (c *ScheduleRestClient) newUpdateScheduleRequest(scheduleID, name, description string,
	useSchedulingSystem bool, timetable Timetable, parameters map[string]string) (*http.Request, error) {

	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("schedule/%s", scheduleID))
	if err != nil {
		return nil, err
	}

	actor := "current"
	if useSchedulingSystem {
		actor = "system"
	}

	var bodyReader io.Reader

	var body = struct {
		Name             string            `json:"name,omitempty"`
		Description      string            `json:"description,omitempty"`
		AttributionActor string            `json:"attribution-actor,omitempty"`
		Parameters       map[string]string `json:"parameters,omitempty"`
		Timetable        Timetable         `json:"timetable,omitempty"`
	}{
		Name:             name,
		Description:      description,
		AttributionActor: actor,
		Parameters:       parameters,
		Timetable:        timetable,
	}
	buf, err := json.Marshal(body)

	if err != nil {
		return nil, err
	}

	bodyReader = bytes.NewReader(buf)

	return c.newHTTPRequest("PATCH", queryURL.String(), bodyReader)
}

// Builds a request to delete an existing schedule.
func (c *ScheduleRestClient) newDeleteScheduleRequest(scheduleID string) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("schedule/%s", scheduleID))
	if err != nil {
		return nil, err
	}
	return c.newHTTPRequest("DELETE", queryURL.String(), nil)
}

// Builds a request to list schedules according to params.
func (c *ScheduleRestClient) newListSchedulesRequest(vcs, org, project string, params *listSchedulesParams) (*http.Request, error) {
	var err error
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("project/%s/%s/%s/schedule", vcs, org, project))
	if err != nil {
		return nil, err
	}

	urlParams := url.Values{}
	if params.PageToken != nil {
		urlParams.Add("page-token", *params.PageToken)
	}

	queryURL.RawQuery = urlParams.Encode()

	return c.newHTTPRequest("GET", queryURL.String(), nil)
}

// Returns a new blank API request with boilerplate headers.
func (c *ScheduleRestClient) newHTTPRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Add("circle-token", c.token)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", version.UserAgent())
	commandStr := header.GetCommandStr()
	if commandStr != "" {
		req.Header.Add("Circleci-Cli-Command", commandStr)
	}
	return req, nil
}

// Verifies that the REST API exists and has the necessary endpoints
// to interact with schedules.
func (c *ScheduleRestClient) EnsureExists() error {
	queryURL, err := url.Parse(c.server)
	if err != nil {
		return err
	}
	queryURL, err = queryURL.Parse("openapi.json")
	if err != nil {
		return err
	}
	req, err := c.newHTTPRequest("GET", queryURL.String(), nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("API v2 test request failed.")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	var respBody struct {
		Paths struct {
			ScheduleEndpoint interface{} `json:"/schedule"`
		}
	}
	if err := json.Unmarshal(bodyBytes, &respBody); err != nil {
		return err
	}

	if respBody.Paths.ScheduleEndpoint == nil {
		return errors.New("No schedule endpoint exists")
	}

	return nil
}

// Returns a new client satisfying the api.ScheduleInterface interface
// via the REST API.
func NewScheduleRestClient(config settings.Config) (*ScheduleRestClient, error) {
	serverURL, err := config.ServerURL()
	if err != nil {
		return nil, err
	}

	client := &ScheduleRestClient{
		token:  config.Token,
		server: serverURL.String(),
		client: config.HTTPClient,
	}

	return client, nil
}
