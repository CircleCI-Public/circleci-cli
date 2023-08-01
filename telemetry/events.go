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

func createEventFromCommandInfo(name string, cmdInfo CommandInfo) Event {
	properties := map[string]interface{}{}
	for key, value := range cmdInfo.LocalArgs {
		properties[fmt.Sprintf("cmd.flag.%s", key)] = value
	}
	properties["has_been_executed"] = false

	return Event{
		Object:     fmt.Sprintf("cli-%s", name),
		Action:     cmdInfo.Name,
		Properties: properties,
	}
}

func errorToProperties(err error) map[string]interface{} {
	properties := map[string]interface{}{
		"has_been_executed": true,
	}

	if err != nil {
		properties["error"] = err.Error()
	}
	return properties
}

func CreateSetupEvent(isServerCustomer bool) Event {
	return Event{
		Object: "cli-setup",
		Action: "setup",
		Properties: map[string]interface{}{
			"is_server_customer": isServerCustomer,
			"has_been_executed":  true,
		},
	}
}

func CreateVersionEvent(version string) Event {
	return Event{
		Object: "cli-version",
		Action: "version",
		Properties: map[string]interface{}{
			"version":           version,
			"has_been_executed": true,
		},
	}
}

func CreateUpdateEvent(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("update", cmdInfo)
}

func CreateDiagnosticEvent(err error) Event {
	return Event{
		Object: "cli-diagnostic", Action: "diagnostic", Properties: errorToProperties(err),
	}
}

func CreateFollowEvent(err error) Event {
	return Event{
		Object: "cli-follow", Action: "follow", Properties: errorToProperties(err),
	}
}

func CreateOpenEvent(err error) Event {
	return Event{Object: "cli-open", Action: "open", Properties: errorToProperties(err)}
}

func CreateCompletionCommand(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("completion", cmdInfo)
}

func CreateConfigEvent(cmdInfo CommandInfo, err error) Event {
	event := createEventFromCommandInfo("config", cmdInfo)
	if err != nil {
		event.Properties["error"] = err.Error()
		event.Properties["has_been_executed"] = true
	}
	return event
}

func CreateLocalExecuteEvent(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("local", cmdInfo)
}

func CreateNamespaceEvent(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("namespace", cmdInfo)
}

func CreateOrbEvent(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("orb", cmdInfo)
}

func CreatePolicyEvent(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("policy", cmdInfo)
}

func CreateRunnerInstanceEvent(cmdInfo CommandInfo, err error) Event {
	event := createEventFromCommandInfo("runner-instance", cmdInfo)
	if err != nil {
		event.Properties["error"] = err.Error()
		event.Properties["has_been_executed"] = true
	}
	return event
}

func CreateRunnerResourceClassEvent(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("runner-resource-class", cmdInfo)
}

func CreateRunnerToken(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("runner-resource-class", cmdInfo)
}

func CreateInfoEvent(cmdInfo CommandInfo, err error) Event {
	event := createEventFromCommandInfo("info", cmdInfo)
	if err != nil {
		event.Properties["error"] = err.Error()
		event.Properties["has_been_executed"] = true
	}
	return event
}

func CreateChangeTelemetryStatusEvent(action string, origin string, err error) Event {
	event := Event{
		Object: "cli-telemetry",
		Action: action,
		Properties: map[string]interface{}{
			"origin": origin,
		},
	}
	if err != nil {
		event.Properties["error"] = err.Error()
	}
	return event
}
