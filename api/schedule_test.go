package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/mock"
)

// Returns a static mock schedule.
func mockSchedule() Schedule {
	return Schedule{
		ID:          "07f08dea-de06-48d4-9b47-9639229b7d24",
		ProjectSlug: "github/test-org/test-project",
		Name:        "test-schedule",
		Description: "A test schedule",
		Timetable: Timetable{
			PerHour:    1,
			HoursOfDay: []uint{4, 8, 15, 16, 23},
			DaysOfWeek: []string{"MON", "THU", "SAT"},
		},
		Actor: Actor{
			ID:    "807d18e2-a8e2-4b1e-8a54-18da6e0cc478",
			Login: "test-actor",
			Name:  "T. Actor",
		},
		Parameters: map[string]string{
			"test": "parameter",
		},
	}
}

// Returns the string representation of the static mock schedule.
func mockScheduleString() string {
	schedule := mockSchedule()
	rv, err := json.Marshal(schedule)
	if err != nil {
		panic("Failed to serialise mock schedule")
	}
	return string(rv)
}

// Takes a number of schedules and formats them as if they were
// returned as a page of our paginated API. No next page tokens
// included at this point.
func formatListResponse(schedules []Schedule) string {
	schedule := mockSchedule()
	var resp = struct {
		Items         []Schedule `json:"items"`
		NextPageToken string     `json:"next_page_token,omitempty"`
	}{
		Items: []Schedule{schedule},
	}
	serialized, err := json.Marshal(resp)
	if err != nil {
		panic("Failed to serialise mock list response")
	}
	return string(serialized)
}

func TestSchedules(t *testing.T) {
	mockFn := func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://circleci.com/api/v2/project/github/test-org/test-project/schedule" {
			panic(fmt.Sprintf("unexpected url: %s", r.URL.String()))
		}
		if r.Method != http.MethodGet {
			panic(fmt.Sprintf("unexpected method: %s", r.Method))
		}
		return mock.NewHTTPResponse(200, formatListResponse([]Schedule{mockSchedule()})), nil
	}
	httpClient := mock.NewHTTPClient(mockFn)
	restClient := ScheduleRestClient{
		server: "https://circleci.com/api/v2/",
		client: httpClient,
	}

	t.Run("Get all schedules for a project", func(t *testing.T) {
		schedules, err := restClient.Schedules("github", "test-org", "test-project")
		assert.NilError(t, err)
		assert.DeepEqual(t, mockSchedule(), (*schedules)[0])
	})
}

func TestScheduleByID(t *testing.T) {
	mockFn := func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://circleci.com/api/v2/schedule/07f08dea-de06-48d4-9b47-9639229b7d24" {
			panic(fmt.Sprintf("unexpected url: %s", r.URL.String()))
		}
		if r.Method != http.MethodGet {
			panic(fmt.Sprintf("unexpected method: %s", r.Method))
		}
		return mock.NewHTTPResponse(200, mockScheduleString()), nil
	}
	httpClient := mock.NewHTTPClient(mockFn)
	restClient := ScheduleRestClient{
		server: "https://circleci.com/api/v2/",
		client: httpClient,
	}

	t.Run("Get a schedule by ID", func(t *testing.T) {
		schedule := mockSchedule()
		gotten, err := restClient.ScheduleByID(schedule.ID)
		assert.NilError(t, err)
		assert.DeepEqual(t, schedule, *gotten)
	})
}

func TestScheduleByName(t *testing.T) {
	mockFn := func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://circleci.com/api/v2/project/github/test-org/test-project/schedule" {
			panic(fmt.Sprintf("unexpected url: %s", r.URL.String()))
		}
		if r.Method != http.MethodGet {
			panic(fmt.Sprintf("unexpected method: %s", r.Method))
		}
		return mock.NewHTTPResponse(200, formatListResponse([]Schedule{mockSchedule()})), nil
	}
	httpClient := mock.NewHTTPClient(mockFn)
	restClient := ScheduleRestClient{
		server: "https://circleci.com/api/v2/",
		client: httpClient,
	}

	t.Run("Get a schedule by name", func(t *testing.T) {
		schedule := mockSchedule()
		gotten, err := restClient.ScheduleByName("github", "test-org", "test-project", schedule.Name)
		assert.NilError(t, err)
		assert.DeepEqual(t, schedule, *gotten)
	})
}

func TestDeleteSchedule(t *testing.T) {
	mockFn := func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://circleci.com/api/v2/schedule/07f08dea-de06-48d4-9b47-9639229b7d24" {
			panic(fmt.Sprintf("unexpected url: %s", r.URL.String()))
		}
		if r.Method != "DELETE" {
			panic(fmt.Sprintf("unexpected method: %s", r.Method))
		}
		return mock.NewHTTPResponse(200, "{\"message\": \"okay\"}"), nil
	}
	httpClient := mock.NewHTTPClient(mockFn)
	restClient := ScheduleRestClient{
		server: "https://circleci.com/api/v2/",
		client: httpClient,
	}

	t.Run("Delete a schedule", func(t *testing.T) {
		err := restClient.DeleteSchedule("07f08dea-de06-48d4-9b47-9639229b7d24")
		assert.NilError(t, err)
	})
}

func TestCreateSchedule(t *testing.T) {
	mockFn := func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://circleci.com/api/v2/project/github/test-org/test-project/schedule" {
			panic(fmt.Sprintf("unexpected url: %s", r.URL.String()))
		}
		if r.Method != "POST" {
			panic(fmt.Sprintf("unexpected method: %s", r.Method))
		}
		return mock.NewHTTPResponse(201, mockScheduleString()), nil
	}
	httpClient := mock.NewHTTPClient(mockFn)
	restClient := ScheduleRestClient{
		server: "https://circleci.com/api/v2/",
		client: httpClient,
	}

	t.Run("Create a schedule", func(t *testing.T) {
		schedule := mockSchedule()
		created, err := restClient.CreateSchedule("github", "test-org", "test-project",
			schedule.Name, schedule.Description, true, schedule.Timetable, schedule.Parameters)
		assert.NilError(t, err)
		assert.DeepEqual(t, schedule, *created)
	})
}

func TestUpdateSchedule(t *testing.T) {
	mockFn := func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://circleci.com/api/v2/schedule/07f08dea-de06-48d4-9b47-9639229b7d24" {
			panic(fmt.Sprintf("unexpected url: %s", r.URL.String()))
		}
		if r.Method != "PATCH" {
			panic(fmt.Sprintf("unexpected method: %s", r.Method))
		}
		return mock.NewHTTPResponse(200, mockScheduleString()), nil
	}
	httpClient := mock.NewHTTPClient(mockFn)
	restClient := ScheduleRestClient{
		server: "https://circleci.com/api/v2/",
		client: httpClient,
	}

	t.Run("Update a schedule", func(t *testing.T) {
		schedule := mockSchedule()
		updated, err := restClient.UpdateSchedule(schedule.ID, schedule.Name, schedule.Description,
			false, schedule.Timetable, schedule.Parameters)
		assert.NilError(t, err)
		assert.DeepEqual(t, schedule, *updated)
	})
}
