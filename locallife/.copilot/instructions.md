# GitHub Copilot Instructions for go-http-scaffold

## Core Architecture Principles

This Go HTTP API scaffold follows these principles:

### 1. Dependency Injection (Constructor Pattern)

```go
//  Good
func NewHandler(store db.Store, config util.Config) *Handler {
    return &Handler{store: store, config: config}
}

//  Avoid
var globalStore db.Store
```

### 2. Interface-Based Design

- Depend on `db.Store` interface, not concrete implementations
- Enables easy testing with mocks
- Allows swapping implementations (PostgreSQL  MongoDB)

### 3. HTTP Status Code Mapping

- 400: Invalid input (validation failure)
- 401: Authentication failure
- 403: Authorization failure (forbidden)
- 404: Resource not found
- 409: Conflict (duplicate, constraint violation)
- 500: Server error

### 4. Error Handling Pattern

Always check errors:
```go
if err != nil {
    log.Error().Err(err).Msg("context message")
    return nil, err
}
```

### 5. Input Validation

Use struct tags for validation:
```go
type Request struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=6"`
}
```

## Architecture Layers

```
Handler Layer (api/*.go)
     Parse input, validate, call service
Business Logic Layer
     Core application logic
Repository Layer (db.Store interface)
     Database operations
Database (PostgreSQL)
```

## Testing

- Use table-driven tests
- Mock the db.Store interface
- Test both success and failure paths
- One test file per handler: `*_test.go`

## Key Files to Reference

- `api/server.go` - Server definition and routing
- `api/user.go` - Example handler implementation
- `db/sqlc/store.go` - Store interface definition
- `token/paseto_maker.go` - Token implementation
- `util/config.go` - Configuration management

## When Generating Code

Include these instructions:
1. Follow the architecture layers above
2. Use constructor injection for dependencies
3. Implement table-driven tests
4. Return appropriate HTTP status codes
5. Add detailed error logging

## When Reviewing Code

Check for:
-  Constructor injection used
-  Errors are handled
-  Input validated
-  Tests included
-  No global state
-  Following naming conventions
