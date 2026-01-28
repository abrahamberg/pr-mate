# PRMate - GitHub Webhook Handler with Copilot Integration

## Project Overview
PRMate is a Go web application that receives GitHub webhooks and processes them using the GitHub Copilot SDK. The application analyzes pull requests, issues, and push events to provide intelligent insights and suggestions.

## Architecture Principles

### Code Organization
- **Clean Architecture**: Separation of concerns with distinct layers (handlers, services, config)
- **Package Structure**: Organized by feature/domain in `internal/` directory
- **Single Responsibility**: Each package and function has one clear purpose
- **Dependency Injection**: Services are injected rather than created in handlers

### Go Best Practices
- **Error Handling**: Always check and wrap errors with context
- **Graceful Shutdown**: Implement proper signal handling and cleanup
- **Middleware**: Use middleware for cross-cutting concerns (logging, auth)
- **Configuration**: Environment-based configuration with sensible defaults
- **Concurrency Safety**: Use mutexes for shared state protection

### Code Style
- **Short Functions**: Break complex logic into smaller, testable functions
- **Clear Naming**: Use descriptive names that explain purpose
- **No Comments for Logic**: Code should be self-documenting; functions replace comments
- **Interface Usage**: Define interfaces for testability and flexibility
- **Context Propagation**: Pass context for cancellation and timeouts



## Key Components

### Webhook Handler (`internal/webhook`)
- Receives and validates GitHub webhook requests
- Routes events to appropriate handlers
- Validates HMAC signatures for security
- Supports: pull_request, issues, push, ping events

### Copilot Service (`internal/copilot`)
- Manages Copilot SDK client lifecycle
- Creates sessions for analysis
- Provides analysis for PRs, issues, and commits
- Thread-safe with mutex protection

### Server (`internal/server`)
- HTTP server with graceful shutdown
- Logging middleware for request tracking
- Health check endpoint
- Proper timeout configuration

## Development Guidelines

### Adding New Event Handlers
1. Add event type to webhook handler switch statement
2. Create typed payload struct in `types.go`
3. Implement handler function following single responsibility principle
4. Add corresponding analysis method in copilot service

### Error Handling Pattern
```go
if err != nil {
    return fmt.Errorf("context about what failed: %w", err)
}
```

### Function Naming
- Use verbs for action functions: `handlePullRequest`, `validateSignature`
- Use nouns for constructors: `NewHandler`, `NewService`
- Use `Get` prefix for accessors: `GetConfig`, `GetSession`
- Be specific and descriptive

### Testing Strategy
- Unit tests for business logic functions
- Integration tests for webhook handlers
- Mock Copilot service for testing
- Table-driven tests for multiple scenarios

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GITHUB_TOKEN` | Yes | - | GitHub authentication token |
| `PORT` | No | 8080 | Server port |
| `WEBHOOK_SECRET` | No | - | GitHub webhook secret for signature validation |

## API Endpoints

- `POST /webhook` - GitHub webhook receiver
- `GET /health` - Health check endpoint
- `GET /` - Root endpoint (service info)

## Security Considerations
- Always validate webhook signatures when `WEBHOOK_SECRET` is set
- Use HTTPS in production
- Keep GitHub token secure
- Implement rate limiting for production deployments
- Log security events appropriately

## Code Quality Checklist
- [ ] No functions longer than 50 lines
- [ ] All errors properly wrapped with context
- [ ] No naked returns
- [ ] Exported functions have comments
- [ ] No global mutable state
- [ ] Proper resource cleanup (defer)
- [ ] Context passed through call chains
- [ ] Thread-safe shared state access
