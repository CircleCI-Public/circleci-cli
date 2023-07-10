package telemetry

import "fmt"

// This file contains all the telemetry event constructors
// All the events are referenced in the following file:
// https://circleci.atlassian.net/wiki/spaces/DE/pages/6760694125/CLI+segment+event+tracking
// If you want to add an event, first make sure it appears in this file

type CommandInfo struct {
	Name      string
	LocalArgs map[string]string
}

func localArgsToProperties(cmdInfo CommandInfo) map[string]interface{} {
	properties := map[string]interface{}{}
	for key, value := range cmdInfo.LocalArgs {
		properties[fmt.Sprintf("cmd.flag.%s", key)] = value
	}
	return properties
}

func CreateSetupEvent(isServerCustomer bool) Event {
	return Event{
		Object: "cli-setup",
		Properties: map[string]interface{}{
			"is_server_customer": isServerCustomer,
		},
	}
}

func CreateVersionEvent(version string) Event {
	return Event{
		Object: "cli-version",
		Properties: map[string]interface{}{
			"version": version,
		},
	}
}

func CreateUpdateEvent(cmdInfo CommandInfo) Event {
	return Event{
		Object:     "cli-update",
		Action:     cmdInfo.Name,
		Properties: localArgsToProperties(cmdInfo),
	}
}

func CreateDiagnosticEvent() Event {
	return Event{
		Object: "cli-diagnostic",
	}
}
