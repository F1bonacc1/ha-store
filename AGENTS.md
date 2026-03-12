# AGENTS.md - Coding Guidelines for ha-store

This file provides guidelines for AI agents working on the ha-store Go codebase.

## Build/Test Commands

```bash
# Build the application
make build

# Run all tests
make test                    # Runs: go test -v ./...

# Run a single test file
go test -v ./handlers -run TestHandlePutFile

# Run a single test function
go test -v -run TestFileLifecycle ./...

# Run tests with coverage
make coverage                # Generates coverage.out and displays summary

# Run integration tests (requires NATS or uses embedded)
make test-integration        # Runs: go test -v -tags=integration ./tests/integration/...

# Start/stop Docker dependencies (NATS server)
make docker-up
make docker-down

# Clean build artifacts
make clean
```

## Code Style Guidelines

### Imports
- Group imports in three sections separated by blank lines:
  1. Standard library imports
  2. Third-party/external imports
  3. Internal project imports (github.com/f1bonacc1/ha-store/...)
- Example:
  ```go
  import (
      "context"
      "fmt"
      "net/http"

      "github.com/gin-gonic/gin"
      "github.com/rs/zerolog/log"

      "github.com/f1bonacc1/ha-store/store"
  )
  ```
- Use `goimports` or `gofmt` for formatting

### Naming Conventions
- **Files**: Use `snake_case.go` for filenames (e.g., `files_test.go`)
- **Types**: PascalCase for exported (e.g., `FileHandler`), camelCase for unexported
- **Functions**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase for both exported and unexported
- **Constants**: Use PascalCase or camelCase (no ALL_CAPS)
- **Interfaces**: Single method interfaces use `er` suffix (e.g., `Reader`)
- **Test files**: Suffix with `_test.go`, use `package xxx_test` for black-box tests
- **Test functions**: Start with `Test` prefix, descriptive names (e.g., `TestFileLifecycle`)

### Error Handling
- Use `fmt.Errorf()` with `%w` verb to wrap errors: `fmt.Errorf("failed to save: %w", err)`
- Log errors using zerolog before returning when appropriate:
  ```go
  log.Error().Err(err).Str("path", path).Msg("failed to save file")
  ```
- Check for specific error types (e.g., `jetstream.ErrObjectNotFound`)
- Use `require.NoError(t, err)` in tests for fatal errors
- Use `assert.Equal(t, expected, actual)` for test assertions

### Types and Structs
- Define structs with exported fields using PascalCase
- Include struct tags when needed (JSON, etc.)
- Use explicit types, avoid `interface{}` unless necessary
- Example:
  ```go
  type FileHandler struct {
      Store          *store.Store
      ThrottleSpeed  int64
      UploadDeadline time.Duration
  }
  ```

### Logging
- Use `github.com/rs/zerolog/log` for all logging
- Use structured logging with chained methods:
  ```go
  log.Info().Str("method", method).Int("status", status).Msg("request processed")
  log.Error().Err(err).Str("path", path).Msg("operation failed")
  ```
- Levels: `log.Info()`, `log.Error()`, `log.Warn()`, `log.Fatal()`

### Testing Patterns
- Use `testify/assert` and `testify/require` for assertions
- Use embedded NATS server via `testhelpers.StartEmbeddedNATS(t)`
- Clean up resources with `t.Cleanup()` or `defer`
- Create test helpers like `setupHandlerTest(t)` to reduce boilerplate
- Use `gin.SetMode(gin.TestMode)` in HTTP handler tests
- Use `httptest.NewRecorder()` for HTTP response recording
- Use unique identifiers (timestamps/nanos) to avoid test collisions

### HTTP Handlers (Gin)
- Use `*gin.Context` as the only parameter
- Return appropriate HTTP status codes
- Use `c.JSON()` for JSON responses, `c.Status()` for empty responses
- Access path params with `c.Param("path")`
- Access query params with `c.Query("param")`
- Always trim leading slashes from paths: `strings.TrimPrefix(path, "/")`

### Context Usage
- Use `context.WithTimeout()` for operations with deadlines
- Always call `defer cancel()` immediately after creating context
- Pass context through to underlying operations (NATS, etc.)

### Comments
- Use full sentences with proper punctuation
- Start with the thing being described (function name for func docs)
- Explain "why" not just "what" for complex logic
- Example: `// NewThrottledReader creates a reader that limits throughput...`

## Project Structure

```
├── config/        # Configuration loading (flags + env vars)
├── handlers/      # HTTP handlers (Gin)
├── store/         # NATS/JetStream storage abstraction
├── testhelpers/   # Test utilities (embedded NATS)
├── tests/integration/  # Integration tests with build tag
├── main.go        # Application entry point
├── main_test.go   # Main package tests
└── Makefile       # Build automation
```

## Dependencies

Key external dependencies:
- `github.com/gin-gonic/gin` - HTTP web framework
- `github.com/nats-io/nats.go` - NATS client
- `github.com/rs/zerolog` - Structured logging
- `github.com/stretchr/testify` - Testing utilities

## General Guidelines

- Follow standard Go conventions (Effective Go)
- Use `gofmt` for automatic formatting
- Keep functions focused and small
- Prefer composition over inheritance
- Use goroutines for async operations (fire-and-forget pattern)
- Handle resource cleanup with `defer` consistently
- Module path: `github.com/f1bonacc1/ha-store`
- Minimum Go version: 1.24.2
