package telemetry

// This file contains all the telemetry event constructors
// All the events are referenced in the following file:
// https://circleci.atlassian.net/wiki/spaces/DE/pages/6760694125/CLI+segment+event+tracking
// If you want to add an event, first make sure it appears in this file

func CreateSetupEvent(isServerCustomer bool) Event {
	return Event{
		Object: "cli-setup",
		Action: "called",
		Properties: map[string]interface{}{
			"is_server_customer": isServerCustomer,
		},
	}
}

func CreateVersionEvent() Event {
	return Event{
		Object:     "cli-version",
		Action:     "called",
		Properties: map[string]interface{}{},
	}
}
