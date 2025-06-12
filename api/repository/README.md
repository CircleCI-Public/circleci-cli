# Repository API Module

This module provides access to GitHub repository information through CircleCI's Backend-for-Frontend (BFF) service.

## Overview

The repository API module allows you to fetch GitHub repositories for an organization using the CircleCI BFF service endpoint:

```
GET https://bff.circleci.com/private/soc/github-app/organization/:orgId/repositories
```

## Features

- ✅ Fetch GitHub repositories for an organization
- ✅ Standard CircleCI authentication (Circle-Token header)
- ✅ Comprehensive error handling
- ✅ Full test coverage
- ✅ Consistent with existing CircleCI CLI API patterns

## Usage

### Basic Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/CircleCI-Public/circleci-cli/api/repository"
    "github.com/CircleCI-Public/circleci-cli/settings"
)

func main() {
    // Initialize configuration
    config := settings.Config{
        Token: "your-circleci-token",
    }

    // Create the repository client
    client, err := repository.NewRepositoryRestClient(config)
    if err != nil {
        log.Fatalf("Failed to create repository client: %v", err)
    }

    // Fetch GitHub repositories for an organization
    repositories, err := client.GetGitHubRepositories("your-org-id")
    if err != nil {
        log.Fatalf("Failed to fetch repositories: %v", err)
    }

    // Display the results
    fmt.Printf("Found %d repositories:\n", repositories.TotalCount)
    for _, repo := range repositories.Repositories {
        fmt.Printf("- %s (%s)\n", repo.FullName, repo.Language)
    }
}
```

## API Reference

### Types

#### `Repository`

Represents a GitHub repository with the following fields:

- `ID` (int): GitHub repository ID
- `Name` (string): Repository name
- `FullName` (string): Full repository name (org/repo)
- `Private` (bool): Whether the repository is private
- `HTMLURL` (string): GitHub web URL
- `CloneURL` (string): HTTPS clone URL
- `SSHURL` (string): SSH clone URL
- `Description` (string): Repository description
- `Language` (string): Primary programming language
- `CreatedAt` (string): Creation timestamp
- `UpdatedAt` (string): Last update timestamp
- `PushedAt` (string): Last push timestamp
- `DefaultBranch` (string): Default branch name

#### `GetRepositoriesResponse`

Response wrapper for the repositories endpoint:

- `Repositories` ([]Repository): List of repositories (populated from API array)
- `TotalCount` (int): Total number of repositories (calculated from array length)

**Note**: The BFF API returns a JSON array of repositories directly, not an object. The `GetRepositoriesResponse` struct provides a consistent interface by wrapping the array and calculating the total count.

### Methods

#### `NewRepositoryRestClient(config settings.Config) (*repositoryRestClient, error)`

Creates a new repository REST client with the provided configuration.

#### `GetGitHubRepositories(orgID string) (*GetRepositoriesResponse, error)`

Fetches GitHub repositories for the specified organization ID.

## Authentication

This API uses CircleCI's standard authentication mechanism:

- Set your CircleCI token in the `settings.Config.Token` field
- The token will be sent as a `Circle-Token` header with all requests

## Error Handling

The client provides comprehensive error handling:

- Network errors are wrapped with context
- HTTP errors include status codes and response messages
- JSON parsing errors are clearly identified

## Testing

Run the tests with:

```bash
go test ./api/repository/...
```

The test suite includes:

- Successful API call scenarios
- Error response handling
- Network error scenarios
- Client initialization testing

## Integration

This module follows the same patterns as other CircleCI CLI API modules:

- Uses the standard `settings.Config` for configuration
- Includes proper headers (User-Agent, Circle-Token, etc.)
- Follows Go naming conventions and error handling patterns
- Compatible with the existing CLI architecture
