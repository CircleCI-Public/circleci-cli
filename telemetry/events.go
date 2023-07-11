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

	return Event{
		Object:     fmt.Sprintf("cli-%s", name),
		Action:     cmdInfo.Name,
		Properties: properties,
	}
}

func errorToProperties(err error) map[string]interface{} {
	if err == nil {
		return nil
	}
	return map[string]interface{}{
		"error": err.Error(),
	}
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
	return createEventFromCommandInfo("update", cmdInfo)
}

func CreateDiagnosticEvent(err error) Event {
	return Event{
		Object: "cli-diagnostic", Properties: errorToProperties(err),
	}
}

func CreateFollowEvent(err error) Event {
	return Event{
		Object: "cli-follow", Properties: errorToProperties(err),
	}
}

func CreateOpenEvent(err error) Event {
	return Event{Object: "cli-open", Properties: errorToProperties(err)}
}

func CreateCompletionCommand(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("completion", cmdInfo)
}

func CreateConfigEvent(cmdInfo CommandInfo) Event {
	return createEventFromCommandInfo("config", cmdInfo)
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
