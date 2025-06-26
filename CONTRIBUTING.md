# Contributing to AKS Mentions Bot

We welcome contributions to the AKS Mentions Bot project! This document provides guidelines for contributing.

## Development Setup

1. **Prerequisites**
   - Go 1.21 or later
   - Docker (for containerization)
   - Azure CLI (for deployment)
   - Git

2. **Clone and Setup**
   ```bash
   git clone <repository-url>
   cd aks-mentions-bot
   cp .env.example .env
   go mod tidy
   ```

3. **Run Tests**
   ```bash
   go test ./...
   ```

4. **Build Locally**
   ```bash
   go build -o bin/aks-mentions-bot ./cmd/bot
   ```

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Run `go vet` to check for common issues
- Add comments for exported functions and types
- Write tests for new functionality

## Project Structure

```
├── cmd/bot/                 # Application entry point
├── internal/
│   ├── config/             # Configuration management
│   ├── models/             # Data models
│   ├── monitoring/         # Core monitoring logic
│   ├── notifications/      # Notification services
│   ├── scheduler/          # Task scheduling
│   ├── sources/            # Data source implementations
│   └── storage/            # Data storage
├── infra/                  # Azure infrastructure (Bicep)
├── cmd/                    # Application entrypoints
├── internal/               # Internal packages
├── k8s/                    # Kubernetes manifests  
├── infra/                  # Infrastructure as Code (Bicep)
├── scripts/                # Build and deployment scripts
└── scripts/                # Utility scripts
```

## Adding New Data Sources

To add a new data source:

1. Create a new file in `internal/sources/`
2. Implement the `Source` interface:
   ```go
   type Source interface {
       GetName() string
       FetchMentions(ctx context.Context, keywords []string, since time.Duration) ([]models.Mention, error)
       IsEnabled() bool
   }
   ```
3. Add the source to `monitoring/service.go` in `initializeSources()`
4. Add configuration options to `internal/config/`
5. Write tests for the new source
6. Update documentation

## Testing Guidelines

- Write unit tests for all new functionality
- Use table-driven tests where appropriate
- Mock external dependencies
- Ensure tests are deterministic
- Test error conditions and edge cases

Example test structure:
```go
func TestNewFeature(t *testing.T) {
    tests := []struct {
        name     string
        input    interface{}
        expected interface{}
    }{
        // test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Pull Request Process

1. **Branch Naming**
   - Feature branches: `feature/description`
   - Bug fixes: `fix/description`
   - Code: `/source/description`
   - Tests: `/test/description`

2. **Commit Messages**
   - Use clear, descriptive commit messages
   - Start with a verb (Add, Fix, Update, Remove)
   - Keep first line under 50 characters
   - Add details in body if needed

3. **Before Submitting**
   ```bash
   # Run tests
   go test ./...
   
   # Check formatting
   go fmt ./...
   
   # Check for issues
   go vet ./...
   
   # Build successfully
   go build ./cmd/bot
   ```

4. **PR Requirements**
   - All tests must pass
   - Code must be formatted with `gofmt`
   - Include tests for new functionality
   - Update documentation if needed
   - Describe changes in PR description

## Reporting Issues

When reporting issues, please include:

- Go version (`go version`)
- Operating system
- Clear description of the problem
- Steps to reproduce
- Expected behavior
- Actual behavior
- Relevant logs or error messages

## Feature Requests

For feature requests:

- Check existing issues first
- Describe the use case
- Explain why the feature would be valuable
- Consider implementation complexity
- Discuss potential breaking changes

## Security

- Never commit API keys or secrets
- Use Azure Key Vault for sensitive data
- Follow secure coding practices
- Report security issues privately

## Documentation

- Keep README.md up to date
- Document configuration options
- Include examples in documentation
- Update API documentation for changes

## Release Process

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create release notes
4. Tag the release
5. Update Docker images
6. Deploy to production environments

## Getting Help

- Check existing documentation
- Search existing issues
- Ask questions in discussions
- Contact maintainers for urgent issues

Thank you for contributing to the AKS Mentions Bot!
