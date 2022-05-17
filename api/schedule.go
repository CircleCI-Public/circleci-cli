package api

import (
	"time"
)

type Timetable struct {
	PerHour    uint     `json:"per-hour"`
	HoursOfDay []uint   `json:"hours-of-day"`
	DaysOfWeek []string `json:"days-of-week"`
}

type Actor struct {
	ID    string `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

type Schedule struct {
	ID          string            `json:"id"`
	ProjectSlug string            `json:"project-slug"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Timetable   Timetable         `json:"timetable"`
	Actor       Actor             `json:"actor"`
	Parameters  map[string]string `json:"parameters"`
	CreatedAt   time.Time         `json:"created-at"`
	UpdatedAt   time.Time         `json:"updated-at"`
}

type ScheduleInterface interface {
	Schedules(vcs, org, project string) (*[]Schedule, error)
	ScheduleByID(scheduleID string) (*Schedule, error)
	ScheduleByName(vcs, org, project, name string) (*Schedule, error)
	DeleteSchedule(scheduleID string) error
	CreateSchedule(vcs, org, project, name, description string,
		useSchedulingSystem bool, timetable Timetable,
		parameters map[string]string) (*Schedule, error)
	UpdateSchedule(scheduleID, name, description string,
		useSchedulingSystem bool, timetable Timetable,
		parameters map[string]string) (*Schedule, error)
}
