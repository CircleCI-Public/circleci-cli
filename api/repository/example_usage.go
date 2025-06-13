// Package repository provides an example of how to use the GitHub repositories API endpoint.
// This example demonstrates fetching GitHub repositories for an organization using the BFF service.
package repository

import (
	"fmt"
	"log"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

// Example demonstrates how to use the GitHub repositories API endpoint
func Example() {
	// Initialize configuration
	config := settings.Config{
		Token: "your-circleci-token", // Replace with your actual CircleCI token
		// HTTPClient will be initialized with defaults if not provided
	}

	// Create the repository client
	client, err := NewRepositoryRestClient(config)
	if err != nil {
		log.Fatalf("Failed to create repository client: %v", err)
	}

	// Fetch GitHub repositories for an organization
	// Note: The API returns an array of repositories directly
	orgID := "your-org-id" // Replace with your actual organization ID
	repositories, err := client.GetGitHubRepositories(orgID)
	if err != nil {
		log.Fatalf("Failed to fetch repositories: %v", err)
	}

	// Display the results
	fmt.Printf("Found %d repositories:\n", repositories.TotalCount)
	for i, repo := range repositories.Repositories {
		fmt.Printf("%d. %s\n", i+1, repo.FullName)
		fmt.Printf("   Description: %s\n", repo.Description)
		fmt.Printf("   Language: %s\n", repo.Language)
		fmt.Printf("   Private: %t\n", repo.Private)
		fmt.Printf("   Default Branch: %s\n", repo.DefaultBranch)
		fmt.Printf("   HTML URL: %s\n", repo.HTMLURL)
		fmt.Printf("   Clone URL: %s\n", repo.CloneURL)
		fmt.Printf("   SSH URL: %s\n", repo.SSHURL)
		fmt.Println()
	}
}

// ExampleWithCustomClient demonstrates how to use a custom HTTP client
func ExampleWithCustomClient() {
	// You can customize the HTTP client if needed
	// For example, to set custom timeouts, proxy settings, etc.

	config := settings.Config{
		Token: "your-circleci-token",
		// HTTPClient: &http.Client{
		//     Timeout: 30 * time.Second,
		//     Transport: &http.Transport{
		//         Proxy: http.ProxyFromEnvironment,
		//     },
		// },
	}

	client, err := NewRepositoryRestClient(config)
	if err != nil {
		log.Fatalf("Failed to create repository client: %v", err)
	}

	// Use the client as shown in the Example() function above
	repositories, err := client.GetGitHubRepositories("your-org-id")
	if err != nil {
		log.Fatalf("Failed to fetch repositories: %v", err)
	}

	fmt.Printf("Successfully fetched %d repositories\n", repositories.TotalCount)
}
