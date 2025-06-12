# API Response Format

## Actual BFF API Response

The CircleCI BFF endpoint `https://bff.circleci.com/private/soc/github-app/organization/:orgId/repositories` returns a **JSON array directly**, not an object.

### Example Response

```json
[
  {
    "id": 123456,
    "name": "frontend-app",
    "full_name": "myorg/frontend-app",
    "private": false,
    "html_url": "https://github.com/myorg/frontend-app",
    "clone_url": "https://github.com/myorg/frontend-app.git",
    "ssh_url": "git@github.com:myorg/frontend-app.git",
    "description": "React application for our main product",
    "language": "JavaScript",
    "created_at": "2023-01-01T00:00:00Z",
    "updated_at": "2023-12-01T00:00:00Z",
    "pushed_at": "2023-12-01T12:00:00Z",
    "default_branch": "main"
  },
  {
    "id": 789012,
    "name": "backend-api",
    "full_name": "myorg/backend-api",
    "private": true,
    "html_url": "https://github.com/myorg/backend-api",
    "clone_url": "https://github.com/myorg/backend-api.git",
    "ssh_url": "git@github.com:myorg/backend-api.git",
    "description": "REST API backend service",
    "language": "Go",
    "created_at": "2023-02-01T00:00:00Z",
    "updated_at": "2023-11-15T00:00:00Z",
    "pushed_at": "2023-11-15T15:30:00Z",
    "default_branch": "master"
  }
]
```

## How We Handle It

Our Go client handles this by:

1. **Unmarshaling into []Repository**: We unmarshal directly into a slice of Repository structs
2. **Creating Response Wrapper**: We wrap the array in a GetRepositoriesResponse struct for consistency
3. **Calculating Total Count**: We set TotalCount to len(repositories) since we have all repositories

### Code Flow

```go
// 1. Unmarshal the JSON array directly
var repositories []Repository
if err := json.Unmarshal(bodyBytes, &repositories); err != nil {
    return nil, fmt.Errorf("failed to decode response: %w", err)
}

// 2. Create the response structure
result := &GetRepositoriesResponse{
    Repositories: repositories,
    TotalCount:   len(repositories),
}

return result, nil
```

## Error That Was Fixed

**Before (Incorrect):**

```go
// This failed because API doesn't return an object
var result GetRepositoriesResponse
if err := json.Unmarshal(bodyBytes, &result); err != nil {
    // Error: json: cannot unmarshal array into Go value of type repository.GetRepositoriesResponse
}
```

**After (Correct):**

```go
// This works because we unmarshal the array directly
var repositories []Repository
if err := json.Unmarshal(bodyBytes, &repositories); err != nil {
    return nil, fmt.Errorf("failed to decode response: %w", err)
}
```

## Benefits of This Approach

✅ **Matches API Response**: Correctly handles the actual JSON array format  
✅ **Consistent Interface**: Provides a uniform response structure to callers  
✅ **Error Resilient**: Proper error handling for JSON parsing  
✅ **Future Proof**: Easy to adapt if API format changes
