# Contributing to k8s-postgresql-operator

We welcome contributions to the k8s-postgresql-operator! This document provides guidelines for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Code Style](#code-style)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Commit Message Format](#commit-message-format)
- [Issue Guidelines](#issue-guidelines)
- [Documentation](#documentation)

## Code of Conduct

This project adheres to a code of conduct. By participating, you are expected to uphold this code. Please report unacceptable behavior to the project maintainers.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/your-username/k8s-postgresql-operator.git
   cd k8s-postgresql-operator
   ```
3. **Set up the development environment** (see [docs/local-development.md](docs/local-development.md))
4. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Setup

### Prerequisites

- Go 1.25.6+
- Docker or Podman
- kubectl with access to a Kubernetes cluster
- Make
- HashiCorp Vault (for testing)
- PostgreSQL (for testing)

### Local Environment

```bash
# Install dependencies
go mod download

# Start test environment
make test-env-up

# Setup Vault for development
make test-env-setup-vault-local-k8s

# Run tests to verify setup
make test
```

For detailed setup instructions, see [docs/local-development.md](docs/local-development.md).

## Code Style

### Go Code Standards

We follow standard Go conventions and use automated tools to enforce consistency:

#### Formatting

```bash
# Format code (required before committing)
make fmt

# Run go vet
make vet

# Run linter
make lint

# Auto-fix linting issues where possible
make lint-fix
```

#### Code Organization

- **Package Structure**: Follow the existing package structure
- **Naming Conventions**: Use Go naming conventions (PascalCase for exported, camelCase for unexported)
- **Error Handling**: Always handle errors appropriately
- **Context Usage**: Use context.Context for cancellation and timeouts
- **Logging**: Use structured logging with appropriate log levels

#### Example Code Style

```go
// Good: Proper error handling and logging
func (r *PostgreSQLReconciler) reconcileDatabase(ctx context.Context, postgresql *v1alpha1.Postgresql) error {
    logger := log.FromContext(ctx).WithValues("postgresql", postgresql.Name)
    
    if err := r.validatePostgreSQL(ctx, postgresql); err != nil {
        logger.Error(err, "Failed to validate PostgreSQL instance")
        return fmt.Errorf("validation failed: %w", err)
    }
    
    logger.Info("Successfully reconciled PostgreSQL instance")
    return nil
}

// Bad: No error handling or logging
func (r *PostgreSQLReconciler) reconcileDatabase(postgresql *v1alpha1.Postgresql) {
    r.validatePostgreSQL(postgresql)
}
```

### YAML and Helm Templates

- Use 2-space indentation
- Follow Kubernetes resource naming conventions
- Include appropriate labels and annotations
- Use Helm template functions consistently

### Documentation

- Document all exported functions and types
- Use clear, concise comments
- Include examples in documentation
- Update README.md for significant changes

## Testing Requirements

### Test Coverage

**Minimum test coverage: 90%**

```bash
# Run tests with coverage
make test-coverage

# View coverage report
go tool cover -html=coverage.out
```

### Test Types

#### 1. Unit Tests (Required)

- Test individual functions and methods
- Use mocks for external dependencies
- Fast execution (< 1 second per test)

```go
func TestPostgreSQLController_Reconcile(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(*testing.T) *PostgreSQLController
        request ctrl.Request
        want    ctrl.Result
        wantErr bool
    }{
        {
            name: "successful reconciliation",
            setup: func(t *testing.T) *PostgreSQLController {
                // Setup test controller with mocks
                return &PostgreSQLController{
                    Client: fake.NewClientBuilder().Build(),
                    // ... other mocked dependencies
                }
            },
            request: ctrl.Request{
                NamespacedName: types.NamespacedName{
                    Name:      "test-postgresql",
                    Namespace: "default",
                },
            },
            want:    ctrl.Result{},
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            controller := tt.setup(t)
            got, err := controller.Reconcile(context.TODO(), tt.request)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Reconcile() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

#### 2. Integration Tests (Recommended)

- Test with real PostgreSQL and Vault
- Use test environment setup

```bash
# Run integration tests
make test-integration
```

#### 3. End-to-End Tests (For Major Changes)

- Test complete workflows in Kubernetes
- Required for significant feature additions

```bash
# Run E2E tests
make test-e2e-local
```

### Test Guidelines

- **Test Names**: Use descriptive test names that explain the scenario
- **Table-Driven Tests**: Use table-driven tests for multiple scenarios
- **Setup and Teardown**: Properly clean up resources in tests
- **Error Testing**: Test both success and failure scenarios
- **Timeouts**: Use appropriate timeouts for async operations

## Pull Request Process

### Before Submitting

1. **Run all checks**:
   ```bash
   make fmt vet lint test
   ```

2. **Ensure test coverage**:
   ```bash
   make test-coverage
   # Verify coverage is >= 90%
   ```

3. **Update documentation** if needed

4. **Test your changes** thoroughly:
   ```bash
   make test-integration
   make test-e2e-local  # For significant changes
   ```

### PR Requirements

- **Clear Description**: Explain what the PR does and why
- **Issue Reference**: Link to related issues
- **Test Coverage**: Maintain or improve test coverage
- **Documentation**: Update docs for user-facing changes
- **Changelog**: Add entry to CHANGELOG.md for notable changes

### PR Template

```markdown
## Description
Brief description of the changes.

## Related Issues
Fixes #123

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] E2E tests pass (if applicable)
- [ ] Test coverage >= 90%

## Checklist
- [ ] Code follows the style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if applicable)
```

### Review Process

1. **Automated Checks**: All CI checks must pass
2. **Code Review**: At least one maintainer review required
3. **Testing**: Reviewer may request additional tests
4. **Documentation**: Ensure docs are updated appropriately

## Commit Message Format

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

### Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types

- **feat**: A new feature
- **fix**: A bug fix
- **docs**: Documentation only changes
- **style**: Changes that do not affect the meaning of the code
- **refactor**: A code change that neither fixes a bug nor adds a feature
- **perf**: A code change that improves performance
- **test**: Adding missing tests or correcting existing tests
- **chore**: Changes to the build process or auxiliary tools

### Examples

```bash
# Feature addition
feat(controller): add support for schema ownership management

# Bug fix
fix(vault): handle authentication timeout correctly

# Documentation
docs(readme): update installation instructions

# Breaking change
feat(api)!: change User CRD spec structure

BREAKING CHANGE: User CRD now requires explicit password policy
```

### Scope Guidelines

- **controller**: Changes to controller logic
- **vault**: Vault integration changes
- **webhook**: Webhook validation changes
- **api**: CRD or API changes
- **helm**: Helm chart changes
- **docs**: Documentation changes
- **test**: Test-related changes

## Issue Guidelines

### Bug Reports

Include:
- **Environment**: Kubernetes version, operator version, etc.
- **Steps to Reproduce**: Clear steps to reproduce the issue
- **Expected Behavior**: What should happen
- **Actual Behavior**: What actually happens
- **Logs**: Relevant operator logs
- **Configuration**: Relevant configuration (sanitized)

### Feature Requests

Include:
- **Use Case**: Why is this feature needed?
- **Proposed Solution**: How should it work?
- **Alternatives**: Other solutions considered
- **Additional Context**: Any other relevant information

### Issue Labels

- **bug**: Something isn't working
- **enhancement**: New feature or request
- **documentation**: Improvements or additions to documentation
- **good first issue**: Good for newcomers
- **help wanted**: Extra attention is needed
- **priority/high**: High priority issue
- **priority/low**: Low priority issue

## Documentation

### Types of Documentation

1. **Code Documentation**: Godoc comments for all exported functions
2. **User Documentation**: README, getting started guides
3. **Developer Documentation**: Contributing guidelines, architecture docs
4. **API Documentation**: CRD specifications and examples

### Documentation Standards

- **Clear and Concise**: Use simple, clear language
- **Examples**: Include working examples
- **Up-to-Date**: Keep documentation current with code changes
- **Structured**: Use consistent formatting and organization

### Documentation Updates

Update documentation when:
- Adding new features
- Changing existing behavior
- Fixing bugs that affect user experience
- Updating configuration options
- Changing installation procedures

## Release Process

### Versioning

We use [Semantic Versioning](https://semver.org/):
- **MAJOR**: Incompatible API changes
- **MINOR**: Backwards-compatible functionality additions
- **PATCH**: Backwards-compatible bug fixes

### Release Checklist

1. Update CHANGELOG.md
2. Update version in relevant files
3. Create release tag
4. Build and publish container images
5. Update Helm chart version
6. Create GitHub release with release notes

## Getting Help

- **Documentation**: Check existing documentation first
- **Issues**: Search existing issues before creating new ones
- **Discussions**: Use GitHub Discussions for questions
- **Code Review**: Ask for help in PR comments

## Recognition

Contributors will be recognized in:
- CHANGELOG.md for significant contributions
- GitHub contributors list
- Release notes for major contributions

Thank you for contributing to k8s-postgresql-operator!