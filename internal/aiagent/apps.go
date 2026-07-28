package aiagent

import "github.com/nika-framework/nika-cli/internal"

// loadApps lists the workspace app names for the chat header.
func loadApps(dir string) ([]string, error) {
	workspace, err := internal.LoadWorkspaceAt(dir)
	if err != nil {
		return nil, err
	}
	if !workspace.Microservice {
		return nil, nil
	}
	return workspace.AppNames(), nil
}
