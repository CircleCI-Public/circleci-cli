# Enhanced Init Command with Repository Selection

The `circleci init` command now includes automatic organization and repository discovery and selection, making it much easier to set up new CircleCI projects.

## New Repository Selection Features

### 🔍 **Automatic Organization and Repository Discovery**

When creating a new project, the init command will:

1. Fetch a list of organizations to which you have access
2. Present them in an interactive selection menu
3. Automatically extract the organization ID for you
4. Fetch a list of repositories for the selected organization
5. Present them in an interactive selection menu
6. Automatically extract the repository ID for you

```
🔍 Fetching available organizations...
✅ Found 3 organizations
? Select an organization:
  > myorg (My Organization) - Main development organization
    client-org (Client Organization) - External client projects
    open-source (Open Source Projects) - Community contributions
    📝 Enter organization/repository manually
```

```
🔍 Fetching available repositories...
✅ Found 25 repositories
? Select a repository:
  > myorg/frontend-app (JavaScript) - React application for our main product
    myorg/backend-api (Go) - REST API backend service
    myorg/mobile-app (Swift) - iOS mobile application
    myorg/data-pipeline (Python) - ETL pipeline for analytics
    myorg/infrastructure (HCL) - Terraform infrastructure as code
    📝 Enter repository ID manually
```

### 🔄 **Smart Fallbacks**

The command gracefully handles various scenarios:

- **API unavailable**: Falls back to manual repository ID input
- **No repositories found**: Prompts for manual input
- **Organization ID missing**: Uses manual input mode
- **User preference**: Always includes manual input option

## Usage Examples

### Basic Interactive Mode

```bash
circleci init
```

This will guide you through:

1. Organization selection (with automatic org ID extraction)
2. Repository selection (with automatic repository discovery)
3. Project, pipeline, and trigger creation

### With Organization Specified

```bash
circleci init github/myorg
```

This will:

1. Use the specified organization
2. Automatically fetch and display repositories for selection
3. Continue with project setup

### Mixed Interactive and Manual

```bash
circleci init github/myorg --project-name myproject
```

This will:

1. Use the specified organization and project name
2. Show repository selection menu
3. Continue with remaining prompts

## Repository Selection Flow

1. **Organization ID Extraction**:

   - Automatically extracted from the collaborators API
   - Used to call the GitHub repositories BFF endpoint

2. **Repository Fetching**:

   - Calls `https://bff.circleci.com/private/soc/github-app/organization/:orgId/repositories`
   - Retrieves comprehensive repository information

3. **Smart Display**:

   - Shows repository name, language, and description
   - Limits display to first 50 repositories for usability
   - Truncates long descriptions to keep menu readable

4. **Automatic ID Extraction**:
   - Extracts GitHub repository ID automatically
   - No need to manually look up repository IDs

## Error Handling

The enhanced init command handles errors gracefully:

```bash
⚠️  Unable to fetch repositories from GitHub (organization not found)
📝 Falling back to manual repository ID input...
```

```bash
📝 Organization ID not available, using manual repository ID input...
```

```bash
📝 No repositories found for this organization. Please enter repository ID manually...
```

## Benefits

✅ **Improved User Experience**: No more manual repository ID lookup
✅ **Visual Repository Browser**: See all your repositories with descriptions
✅ **Robust Fallbacks**: Always works, even when API is unavailable
✅ **Consistent Integration**: Uses existing CircleCI authentication
✅ **Fast Setup**: Reduces project setup time significantly

## Technical Details

- **API Endpoints**:

```
# Organization lookup
https://circleci.com/api/v2/me/collaborations

# Repository lookup
https://bff.circleci.com/private/soc/github-app/organization/:orgId/repositories
```

- **Authentication**: Uses standard CircleCI token via `Circle-Token` header
- **Error Handling**: Comprehensive error handling with user-friendly messages
- **Performance**: Efficient caching and smart pagination
- **Compatibility**: Works with both GitHub and CircleCI VCS types
