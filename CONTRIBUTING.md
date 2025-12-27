# Contributing to Patrol

Thank you for your interest in contributing to Patrol! This document provides guidelines and instructions for contributing to this project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Reporting Issues](#reporting-issues)
- [Security Issues](#security-issues)
- [Development Setup](#development-setup)
- [Code Style](#code-style)
- [Testing Requirements](#testing-requirements)
- [Commit Guidelines](#commit-guidelines)
- [Pull Request Process](#pull-request-process)
- [Documentation Requirements](#documentation-requirements)
- [Security Considerations](#security-considerations)
- [License](#license)

## Code of Conduct

This project adheres to a Code of Conduct that all contributors are expected to follow. Please be respectful, inclusive, and considerate in all interactions. We are committed to providing a welcoming and harassment-free experience for everyone.

## Reporting Issues

### Bug Reports

When reporting bugs, please include:

- **Description**: A clear and concise description of the bug
- **Steps to Reproduce**: Detailed steps to reproduce the behavior
- **Expected Behavior**: What you expected to happen
- **Actual Behavior**: What actually happened
- **Environment**:
  - Patrol version (`patrol version`)
  - Go version (`go version`)
  - Operating system and version
  - Vault/OpenBao version and configuration (if applicable)
- **Logs**: Relevant log output (with sensitive information redacted)
- **Additional Context**: Any other information that might be helpful

### Feature Requests

For feature requests, please describe:

- The use case and problem you're trying to solve
- Your proposed solution
- Any alternatives you've considered
- How this would benefit other users

## Security Issues

**CRITICAL: Do NOT report security vulnerabilities through public GitHub issues.**

Patrol handles authentication tokens and secrets, making security paramount. If you discover a security vulnerability:

1. **Email**: Send details to the project maintainers privately
2. **Include**:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if you have one)
3. **Wait**: Allow maintainers time to respond before public disclosure
4. **Responsible Disclosure**: Follow coordinated disclosure practices

Security issues will be prioritized and addressed with urgency.

## Development Setup

### Prerequisites

- **Go 1.25** - [Install Go](https://golang.org/doc/install)
- **Git** - For version control
- **GPG Key** - For commit signing (required)
- **Make** - For build automation (required)

### Initial Setup

1. **Fork and Clone**

   ```bash
   git clone https://github.com/YOUR_USERNAME/patrol.git
   cd patrol
   ```

2. **Install Dependencies**

   ```bash
   make deps
   make verify-deps
   ```

3. **Install Development Tools**

   ```bash
   make install-dev-tools
   ```

4. **Configure Git for Signing**

   ```bash
   # Set your GPG key
   git config user.signingkey YOUR_GPG_KEY_ID

   # Enable commit signing by default
   git config commit.gpgsign true
   ```

5. **Build the Project**

   ```bash
   make build
   ```

6. **Run Tests**

   ```bash
   make test
   ```

### Development Workflow

```bash
# Create a feature branch
git checkout -b feature/your-feature-name

# Make changes and test
make build
make test

# Format code
make fmt

# Run linters and security scan
make lint
make security

# Commit with sign-off and GPG signature
git commit -S -s -m "feat: add your feature"

# Push and create PR
git push origin feature/your-feature-name
```

## Code Style

### General Guidelines

- **Go Standard**: Follow [Effective Go](https://golang.org/doc/effective_go.html) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- **Formatting**: All code must be formatted with `gofmt -s`
- **Naming**: Use clear, descriptive names following Go conventions
- **Comments**: Export all public functions, types, and packages with godoc-style comments
- **Error Handling**: Always check and handle errors appropriately
- **Simplicity**: Prefer simple, readable code over clever solutions

### Required Tools

All tools are managed through Make targets:

1. **Format code**: `make fmt`
   - Runs `gofmt -s -w .` and `goimports -w .`
   - Must pass with no errors

2. **Lint code**: `make lint`
   - Runs `golangci-lint run --timeout=5m`
   - Must pass with no errors. Configuration is in `.golangci.yaml`.

3. **Security scan**: `make security`
   - Runs `gosec -fmt text ./...`
   - Must pass with no HIGH or MEDIUM severity issues.

### Code Organization

- Keep functions small and focused (prefer < 50 lines)
- Group related functionality in packages
- Minimize dependencies between packages
- Use interfaces for testability and flexibility
- Avoid global state where possible

## Testing Requirements

Testing is mandatory for all contributions. Patrol handles sensitive authentication tokens, so comprehensive testing is critical.

### Unit Tests

- **Required**: All new features and bug fixes must include unit tests
- **Coverage**: Minimum 80% code coverage for all packages
- **Naming**: Test files must end with `_test.go`
- **Table-Driven**: Use table-driven tests where appropriate
- **Mocking**: Mock external dependencies

Example:

```go
func TestTokenRefresh(t *testing.T) {
    tests := []struct {
        name    string
        token   string
        wantErr bool
    }{
        {"valid token", "hvs.validtoken", false},
        {"invalid token", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := RefreshToken(tt.token)
            if (err != nil) != tt.wantErr {
                t.Errorf("RefreshToken() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Tests

- **Required**: For features that interact with external systems (Vault, OpenBao, filesystem, OS)
- **Build Tags**: Use `// +build integration` to separate from unit tests
- **Cleanup**: Always clean up resources (use `t.Cleanup()`)
- **Documentation**: Document any setup requirements

### Security-Critical Code

Code that handles authentication, tokens, secrets, or encryption requires:

- **90%+ code coverage**
- **Security review** by maintainers
- **Integration tests** with real Vault/OpenBao instances
- **Negative tests** for attack scenarios
- **Fuzz tests** where applicable

### Running Tests

```bash
# Run all unit tests
make test

# Run with coverage report
make coverage

# Run integration tests (requires Docker)
make test-integration

# Run all tests with infrastructure management
make integration
```

### Coverage Requirements

- **Overall**: Minimum 80% coverage
- **Security-Critical**: Minimum 90% coverage
- **New Code**: Should not decrease overall coverage
- **PR Checks**: Coverage is automatically checked in CI

## Commit Guidelines

Patrol uses [Conventional Commits](https://www.conventionalcommits.org/) with additional security requirements.

### Commit Format

```
<type>: <description>

[optional body]

[optional footer(s)]
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation only
- **style**: Formatting, missing semicolons, etc. (no code change)
- **refactor**: Code change that neither fixes a bug nor adds a feature
- **perf**: Performance improvement
- **test**: Adding or updating tests
- **chore**: Maintenance tasks, dependency updates
- **security**: Security-related changes
- **ci**: CI/CD configuration changes

### Rules

1. **Title**:
   - Maximum 50 characters
   - Lowercase only
   - No period at the end
   - Imperative mood ("add" not "added" or "adds")

2. **Body** (optional but recommended):
   - Wrap at 72 characters
   - Explain what and why, not how
   - Separate from title with blank line

3. **Sign-off** (REQUIRED):
   - All commits must include `Signed-off-by: Your Name <your.email@example.com>`
   - Use `git commit -s` to automatically add
   - Indicates you agree to the Developer Certificate of Origin (DCO)

4. **GPG Signature** (REQUIRED):
   - All commits must be GPG-signed
   - Use `git commit -S` to sign
   - Verifies commit authenticity

### Examples

Good commits:

```bash
# Simple feature
git commit -S -s -m "feat: add token auto-renewal"

# With body
git commit -S -s -m "fix: prevent token leak in logs

Tokens were being logged in debug mode. This removes
sensitive token data from all log output.

Fixes #123"

# Breaking change
git commit -S -s -m "feat: change config file format

BREAKING CHANGE: Config files now use TOML instead of YAML.
Run 'patrol migrate-config' to convert existing configs."
```

Bad commits:

```bash
# Too vague
git commit -m "fix stuff"

# Not lowercase
git commit -m "feat: Add Feature"

# Has period
git commit -m "fix: bug fix."

# Not signed off or GPG signed (will be rejected)
git commit -m "feat: add feature"
```

### Developer Certificate of Origin (DCO)

By signing off your commits, you certify that:

- You wrote the code or have the right to submit it
- You agree to the project license (MIT)
- Your contribution is provided under the project license

This is legally important for open source projects.

## Pull Request Process

### Before Submitting

1. **Test**: Ensure all tests pass (`make test`)
2. **Lint**: Run `make lint` and `make security` with no errors
3. **Format**: Run `make fmt`
4. **Coverage**: Check that coverage meets requirements (`make coverage`)
5. **Documentation**: Update relevant documentation
6. **Sign**: All commits must be signed-off and GPG-signed
7. **Rebase**: Rebase on latest main branch

### PR Template

When opening a PR, include:

- **Description**: What does this PR do?
- **Motivation**: Why is this change needed?
- **Testing**: How was this tested?
- **Screenshots**: For UI changes (if applicable)
- **Checklist**:
  - [ ] Tests added/updated
  - [ ] Documentation updated
  - [ ] All commits signed-off (`-s`)
  - [ ] All commits GPG-signed (`-S`)
  - [ ] Linters pass
  - [ ] Coverage requirements met
  - [ ] Security considerations addressed

### Review Process

1. **Automated Checks**: CI must pass (tests, linting, coverage)
2. **Code Review**: At least one maintainer approval required
3. **Security Review**: Required for security-critical changes
4. **Testing**: Maintainers may test locally
5. **Discussion**: Address review comments
6. **Approval**: Once approved, maintainers will merge

### After Submission

- Respond to review comments promptly
- Update PR based on feedback
- Keep PR up to date with main branch
- Be patient and respectful

## Documentation Requirements

### Code Documentation

- **All exported items**: Functions, types, constants, variables must have godoc comments
- **Package comments**: Each package needs a package-level comment
- **Complex logic**: Add inline comments for non-obvious code
- **Examples**: Provide examples for public APIs

Example:

```go
// TokenManager handles the lifecycle of Vault authentication tokens.
// It provides functionality for creating, refreshing, and revoking tokens
// while ensuring secure storage and handling.
type TokenManager struct {
    // fields...
}

// RefreshToken attempts to refresh an existing Vault token.
// It returns the new token TTL on success, or an error if the
// refresh fails or the token is not renewable.
func (tm *TokenManager) RefreshToken(ctx context.Context) (time.Duration, error) {
    // implementation...
}
```

### User Documentation

Update when adding features or changing behavior:

- **README.md**: For major features or changes
- **Command help**: Update `--help` text for CLI changes
- **Examples**: Add usage examples
- **Configuration**: Document new configuration options

### Changelog

- Significant changes should be noted for release notes
- Maintainers will handle versioning and releases

## Security Considerations

Patrol is critical infrastructure that handles authentication tokens. All contributors must understand and follow security best practices.

### Security Principles

1. **Least Privilege**: Request minimum necessary permissions
2. **Defense in Depth**: Multiple layers of security
3. **Fail Securely**: Default to secure behavior on error
4. **Secure Defaults**: Make the secure option the default
5. **Audit Trail**: Log security-relevant events
6. **Zero Trust**: Validate all inputs and outputs

### Secure Coding Practices

- **Input Validation**: Validate and sanitize all inputs
- **No Secrets in Logs**: Never log tokens, passwords, or sensitive data
- **Secure Storage**: Use OS-provided secure storage mechanisms
- **Memory Safety**: Clear sensitive data from memory when done
- **Error Messages**: Don't leak sensitive information in errors
- **Dependencies**: Keep dependencies updated; scan for vulnerabilities
- **Cryptography**: Use well-established libraries; don't roll your own

### Token Handling

When working with tokens:

- Store tokens securely (encrypted at rest)
- Transmit over secure channels only
- Clear from memory after use
- Never log token values
- Use secure comparison for token validation
- Implement token rotation
- Handle revocation properly

### Code Review Focus

Security-focused reviews will check for:

- Proper error handling
- Input validation
- Secure token storage
- No information leakage
- Proper authentication/authorization
- Secure communication
- Dependency security

### Security Testing

- Write tests for attack scenarios
- Test error conditions
- Verify secure behavior on failures
- Check for information leakage
- Test permission boundaries

## License

By contributing to Patrol, you agree that your contributions will be licensed under the MIT License. The full license text is available in the [LICENSE](LICENSE) file in the repository.

### MIT License Summary

- You grant a permissive license to your contributions
- Others can use, modify, and distribute your code
- Contributions are provided "as is" without warranty
- You retain copyright to your contributions

---

## Questions?

If you have questions about contributing:

- Open a discussion on GitHub
- Review existing issues and PRs
- Contact the maintainers

Thank you for contributing to Patrol! Your efforts help make secure authentication management better for everyone.
