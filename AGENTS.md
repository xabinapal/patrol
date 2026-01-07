# AGENTS.md

## Role and Context
You are an AI coding assistant acting as a **senior Go (Golang) developer** on this project. This means you should leverage deep expertise in Go best practices, design patterns, and the project's domain (Vault/OpenBao secure storage). Before writing or modifying any code, ensure you fully understand the existing codebase and architecture. Always approach tasks with the mindset of an experienced engineer who values code quality, maintainability, and security.

**This is critical infrastructure software.** Patrol manages authentication tokens for secrets management systems. Any bugs, security flaws, or poor design decisions can lead to credential leaks, unauthorized access, or system compromises. Treat every line of code with the gravity it deserves.

## Quality Standards and Verification

### Never Ship Untested Code
- **Every feature must be verified to work** before considering it complete.
- Run all tests (unit and integration) after implementing any feature.
- If tests fail, fix them before moving on.
- If integration tests require infrastructure, spin it up and verify.
- **"It compiles" is not sufficient** – code must be proven to work.

### Code Coverage Requirements
- **Minimum 80% code coverage** for all packages.
- Critical security paths (token handling, token store operations) must have **90%+ coverage**.
- Check coverage after adding tests: `make coverage` (generates coverage.html) or `make test` (generates coverage.out)
- Identify and address coverage gaps proactively.

### Verification Checklist
Before considering any task complete:
1. [ ] All unit tests pass (`make test`) - **must pass without external dependencies**
2. [ ] All integration tests pass (`make test-integration`)
3. [ ] Code compiles without warnings (`make build`)
4. [ ] Linter passes (`make lint`)
5. [ ] Security scanner passes (`make security`)
6. [ ] Code coverage meets requirements (`make coverage`)
7. [ ] **Tests use mocks - no "binary not found" or similar errors in CI**
8. [ ] **All functions that need testability accept options for dependency injection**
9. [ ] Documentation is updated
10. [ ] Changes are committed with proper messages

## Subagent Usage for Complex Tasks

### When to Use Subagents
- **Always use subagents** for complex, multi-step tasks to avoid context loss.
- Use parallel subagents for independent work streams.
- Each subagent should have a clearly defined, focused scope.

### Subagent Patterns
1. **Research Agent**: Explore codebase, understand existing patterns
2. **Implementation Agent**: Write specific feature or package
3. **Test Agent**: Write and verify tests for a component
4. **Review Agent**: Audit code for security, quality, patterns
5. **Documentation Agent**: Write docs, update README, CONTRIBUTING

### Example Subagent Workflow
For implementing a new feature:
```
1. Spawn research agent → understand existing code patterns
2. Spawn implementation agent → write the feature
3. Spawn test agent → write unit and integration tests
4. Spawn review agent → audit the implementation
5. Integrate results, fix issues, commit
```

### Subagent Communication
- Provide clear, detailed prompts with all necessary context.
- Specify expected outputs explicitly.
- Review subagent results critically before integrating.

## Architecture Patterns

### Type Organization
- **`types` package**: Contains core domain types that are shared across packages:
  - `types.Profile`: Connection profile representation (independent of config or storage)
  - `types.Token`: Internal token model (no JSON annotations, used for business logic)
  - These types should NOT have JSON annotations (they're internal models)
  - These types should NOT have methods that belong to other packages (e.g., token store operations)

### Manager Pattern
- **High-level operations use manager structs**:
  - `TokenManager`: Manages all token operations
    - Initialized with `context.Context`, `tokenstore.Store`, and optional `vault.TokenExecutor` (defaults to `vault.NewTokenExecutor()`)
    - Methods accept `*types.Profile` (not `*config.Connection`)
    - Provides: `Get`, `Set`, `Delete`, `HasToken`, `GetStatus`, `Renew`, `Revoke`, `Lookup`
  - `ProfileManager`: Manages all profile operations
    - Initialized with `context.Context` and `*config.Config`
    - Methods return `*types.Profile` (not `*config.Connection`)
    - Provides: `GetCurrent`, `Get`, `List`

### Package Responsibilities
- **`vault` package**: Low-level Vault/OpenBao server interactions
  - `http.go`: `buildHTTPClient()` function - creates HTTP client with TLS configuration from profile. **Always use this function** when making HTTP requests to Vault/OpenBao servers to ensure proper TLS configuration (certificates, skip verify, etc.) is applied.
  - `token.go`: `TokenExecutor` interface and implementation with methods: `GetTokenStatus`, `LookupToken`, `RenewToken`, `RevokeToken`; and JSON response parsing (ParseLoginResponse, ParseLookupResponse); Vault API response types (VaultTokenResponse, VaultTokenLookupData, TokenStatus, etc.)
  - `health.go`: `HealthExecutor` interface and implementation with method: `CheckHealth`; Health status types (HealthStatus)
  - All executor methods accept `*types.Profile` (convert to `*config.Connection` only when needed for proxy)
  - **No convenience functions**: All vault operations must use executors directly for proper dependency injection
  - **Vault API vs Binary**: **Always prefer using the Vault HTTP API directly** over executing binary commands whenever possible. The API is faster, more reliable, easier to test, and doesn't require the Vault binary to be installed. Use binary commands only when the API doesn't support the operation (e.g., complex login flows that require interactive input).

- **`token` package**: High-level token management
  - `manager.go`: TokenManager implementation

- **`tokenstore` package**: Secure token storage backends
  - `store.go`: Store interface, errors, constants (ServicePrefix, TestStoreEnvVar), helper functions (keyFromProfile), and DefaultStore() factory function
  - `keyring.go`: KeyringStore implementation (OS keyring backend) with NewKeyringStore() constructor
  - `file.go`: FileStore implementation (file-based backend for testing) with NewFileStore() constructor
  - Both stores use constructor functions for consistent initialization
  - The package handles key generation from profiles internally
  - DefaultStore() panics if FileStore creation fails when TestStoreEnvVar is set (no fallback to KeyringStore)

- **`profile` package**: High-level profile management
  - `manager.go`: ProfileManager implementation
  - `types.go`: Profile display types (Info, Status) for CLI output

### Type Conversion Patterns
- **Profile ↔ Connection**: Use `types.FromConnection()` and `prof.ToConnection()`
  - `FromConnection()`: Creates a `*types.Profile` from `*config.Connection`
  - `ToConnection()`: Converts `*types.Profile` to `*config.Connection` (only when needed for proxy operations)
- **Vault Response → Token**: Convert `vault.VaultTokenResponse` to `types.Token` in TokenManager
- **Never use `*config.Connection` directly** in application code - always use `*types.Profile`

## Design Principles and Best Practices

### Core Principles
- **KISS (Keep It Simple, Stupid):** Strive for simple, straightforward solutions. Avoid unnecessary complexity or clever hacks; simple code is easier to test and maintain.
- **DRY (Don't Repeat Yourself):** Eliminate duplicate code. If similar logic is used in multiple places, refactor into a reusable function or component.
- **SOLID Principles:** Adhere to SOLID for any design or refactoring:
  - *Single Responsibility:* Each module or component should have one clear purpose.
  - *Open/Closed:* Code should be open to extension but closed to modification.
  - *Liskov Substitution:* Subtypes must be usable wherever their base type is expected.
  - *Interface Segregation:* Define narrow, role-specific interfaces.
  - *Dependency Inversion:* Depend on abstractions (interfaces), not concretions.
- **YAGNI (You Aren't Gonna Need It):** Unless a feature is needed right now, avoid adding speculative generality.
- **Separation of Concerns:** Organize code so that different concerns are in separate packages or layers.

### Enterprise Code Patterns
- **Dependency Injection:** Use interfaces and constructor injection for testability.
  - Functions that interact with external systems MUST accept options/parameters for dependency injection
  - Use the options pattern (e.g., `...proxy.Option`) for flexible configuration
  - Never create concrete dependencies inside functions that need to be testable
- **Vault API Preference:** **Always prefer using the Vault HTTP API directly** over executing binary commands whenever possible.
  - The API is faster (no subprocess overhead), more reliable, easier to test (standard HTTP mocking), and doesn't require the Vault binary to be installed
  - Use `vault.buildHTTPClient(prof)` function whenever making HTTP requests to Vault/OpenBao servers to ensure proper TLS configuration
  - Use binary commands only when the API doesn't support the operation (e.g., complex login flows that require interactive input or file-based authentication methods)
  - All token operations (renew, revoke, lookup) use the API directly - follow this pattern for new operations
- **Manager Pattern:** High-level operations are managed by dedicated manager structs:
  - `TokenManager`: Manages token operations (initialized with context, tokenstore.Store, and optional vault.TokenExecutor)
  - `ProfileManager`: Manages profile operations (initialized with context and config)
  - Managers provide a clean API and encapsulate business logic
- **Constructor Pattern:** All store implementations use constructor functions:
  - `tokenstore.NewKeyringStore()` - creates OS keyring store
  - `tokenstore.NewFileStore(dir)` - creates file-based store (testing only)
  - `vault.NewTokenExecutor()` - creates vault token executor
  - `vault.NewHealthExecutor()` - creates vault health executor
  - This ensures consistent initialization and enables dependency injection
- **Type Separation:** Core domain types (`Profile`, `Token`) are in the `types` package:
  - `types.Profile`: Connection profile representation (not tied to config or storage)
  - `types.Token`: Token representation (internal application model, not Vault API response)
    - Contains only fields actually used by the application: `ClientToken`, `LeaseDuration`, `Renewable`, `ExpiresAt`
    - Metadata fields (policies, accessor, etc.) are available via `vault.TokenStatus` from lookups, not stored in the internal model
  - Vault API response types are in `vault` package (e.g., `vault.VaultTokenResponse`)
- **Configuration Management:** All configurable values should come from config files or environment.
- **Structured Logging:** Use structured logging (not fmt.Printf) for operational visibility.
- **Graceful Degradation:** Handle failures gracefully; never crash on recoverable errors.
- **Circuit Breakers:** For external service calls, implement timeout and retry logic.
- **Metrics and Observability:** Consider adding metrics hooks for monitoring.

## Go Code Style and Conventions

### Idiomatic Go
- Write code that follows Go idioms and style guidelines.
- Handle errors by returning `error` values (not by using panic for flow control).
- Use short receiver names in methods.
- Prefer composition over inheritance.
- Use context.Context for cancellation and timeouts.

### Formatting and Linting
- **Always** format code with `make fmt` before committing.
- Code **must** pass linters with no issues.
- Required tools: `make lint` (golangci-lint), `make security` (gosec)
- Goal: zero linting or formatting errors in every change.

### Go Version
This project targets **Go 1.25.5** (as specified in go.mod). Ensure all code is compatible, using modern features and standard libraries. Avoid deprecated practices.

### Project Structure
```
patrol/
├── cmd/patrol/          # Main entry point
├── internal/            # Private application code
│   ├── cli/             # CLI commands
│   ├── config/          # Configuration management
│   ├── daemon/          # Background service
│   ├── notify/          # Desktop notifications
│   ├── profile/         # Profile management (ProfileManager)
│   ├── proxy/           # Vault CLI proxy
│   ├── token/           # Token management (TokenManager)
│   ├── tokenstore/      # Secure token storage backends (Store interface, KeyringStore, FileStore)
│   ├── types/           # Core domain types (Profile, Token)
│   ├── utils/           # Utility functions
│   ├── vault/           # Vault/OpenBao server interactions
│   └── version/         # Version info
├── test/
│   └── integration/     # Integration tests
├── .github/             # GitHub templates and workflows
└── docs/                # Additional documentation
```

### Error Handling
- Check and handle all errors returned by functions.
- Never ignore an `error` unless there is a documented reason.
- Use `fmt.Errorf` with `%w` for error wrapping.
- Error messages should be lowercase, no punctuation.
- Fail fast on irrecoverable errors, but do not use panic for routine error handling.

### Comments and Documentation
- All **exported** functions, types, and packages *must* have GoDoc comments.
- Write comments to clarify complex logic, not to restate obvious code.
- Package comments should explain the package's purpose and usage.

## Testing Requirements

### Unit Tests
- Every new feature or bug fix requires unit tests.
- Use table-driven tests for multiple scenarios.
- Name test functions clearly: `TestFunction_Scenario`
- Tests must be independent of each other.
- Use the Arrange-Act-Assert pattern.
- **Test file naming**: Test files must match their source files:
  - `manager.go` → `manager_test.go`
  - `token.go` → `token_test.go`
  - `store.go` → `store_test.go` (if needed)
  - `keyring.go` → `keyring_test.go`
  - `file.go` → `file_test.go`
  - `profile.go` → `profile_test.go`
- **No wrapper test files**: Don't create test files for packages that only contain aliases or wrappers.

### Integration Tests
- **Required for all external interactions** (Vault, token store, filesystem).
- Use build tags: `//go:build integration`
- Must be runnable with: `make test-integration`
- Integration tests must be verified to pass before merging.

### Test Infrastructure
- Docker Compose provided for Vault/OpenBao.
- Tests must handle missing infrastructure gracefully (skip, not fail).
- Clean up test resources after each test.

### Mocking

**CRITICAL**: All functions that interact with external systems (executables, filesystem, network) **MUST** accept options for dependency injection to enable testing.

#### Executor Pattern (vault package)
The `vault` package uses executor interfaces for testability:

- **All vault operations are methods on executor interfaces** (`TokenExecutor`, `HealthExecutor`)
- Executors are created via factory functions: `vault.NewTokenExecutor()`, `vault.NewHealthExecutor()`
- **NEVER create convenience functions that instantiate executors internally** - this breaks dependency injection
- Executor methods accept `...proxy.Option` for command runner injection
- Example pattern:
  ```go
  // ✅ CORRECT: Uses executor interface, allows dependency injection
  type TokenManager struct {
      ctx     context.Context
      store   tokenstore.Store
      vault   vault.TokenExecutor  // Interface, can be mocked
  }

  func NewTokenManager(ctx context.Context, store tokenstore.Store, executor vault.TokenExecutor) *TokenManager {
      return &TokenManager{ctx: ctx, store: store, vault: executor}
  }

  func (tm *TokenManager) Renew(prof *types.Profile, increment string, opts ...proxy.Option) (*types.Token, error) {
      tokenStr, err := tm.store.Get(prof)
      if err != nil {
          return nil, err
      }
      vaultResp, err := tm.vault.RenewToken(tm.ctx, prof, tokenStr, increment, opts...)
      // ...
  }

  // ❌ WRONG: Convenience function that creates executor internally
  func RenewToken(ctx context.Context, prof *types.Profile, tokenStr string, increment string, opts ...proxy.Option) (*VaultTokenResponse, error) {
      executor := NewTokenExecutor()  // Breaks dependency injection!
      return executor.RenewToken(ctx, prof, tokenStr, increment, opts...)
  }
  ```

#### CommandRunner Pattern (proxy package)
The `proxy` package uses a `CommandRunner` interface pattern for testability:

- **All functions that execute commands MUST accept `...proxy.Option`** to allow injecting a mock `CommandRunner`
- Functions like `BinaryExists()`, `NewExecutor()` accept options
- **NEVER create a new `CommandRunner` directly** in functions that need to be testable - always accept options
- Executor methods pass options through to proxy functions

#### General Mocking Guidelines
- Use interfaces for external dependencies (filesystem, network, executables, vault operations)
- Provide mock implementations for testing
- Never mock the thing you're testing
- **When adding new functions that interact with external systems, ensure they accept options/parameters for dependency injection**
- If a helper function calls another function that accepts options, **pass those options through** - don't create new dependencies internally
- Manager structs (TokenManager, ProfileManager) accept dependencies in their constructors for testability
- **Vault operations use executor interfaces** (`vault.TokenExecutor`, `vault.HealthExecutor`) - inject mock executors in tests
- **Never create convenience functions that instantiate executors** - always require executors to be passed in

#### Common Pitfalls to Avoid
- ❌ Creating `NewCommandRunner()` inside a function that should be testable
- ❌ Calling `proxy.BinaryExists()` without passing through options from the caller
- ❌ Hardcoding file system operations instead of accepting interfaces
- ❌ Not passing options through call chains (if function A accepts options and calls function B that also accepts options, pass them through)
- ❌ **Creating convenience functions that instantiate executors internally** (e.g., `func RenewToken(...) { executor := NewTokenExecutor(); return executor.RenewToken(...) }`) - this breaks dependency injection
- ❌ Adding JSON annotations to `types.Token` or `types.Profile` (these are internal models, not API responses)
- ❌ Using `*config.Connection` directly in application code (use `*types.Profile` instead, convert with `prof.ToConnection()` only when needed for proxy operations)
- ❌ Creating wrapper files that only alias types (use original types directly from `types` package)
- ❌ Adding methods to types that belong to other packages (e.g., key generation logic belongs in `tokenstore` package, not as a method on `Profile`)
- ❌ Using direct struct initialization (`&KeyringStore{}`) instead of constructor functions (`NewKeyringStore()`)
- ❌ Implementing fallback behavior in DefaultStore() - should panic on FileStore failure when TestStoreEnvVar is set

### Coverage Goals
| Package | Minimum Coverage |
|---------|-----------------|
| internal/tokenstore | 90% |
| internal/token | 90% |
| internal/types | 90% |
| internal/config | 85% |
| internal/profile | 85% |
| internal/vault | 85% |
| internal/proxy | 80% |
| internal/daemon | 80% |
| internal/cli | 70% |

### Testability Requirements
- **All unit tests MUST run without external dependencies** (no vault binary, no network, no filesystem access beyond temp directories)
- **If a test fails in CI with "binary not found" or similar, it means mocking is incomplete**
- **Before committing, verify tests pass in a clean environment** (e.g., CI environment without vault installed)
- Functions that check for external binaries must accept options to use mocked command runners
- When writing tests, always use mocks - never assume external tools are available

## Git and Commit Best Practices

### Commit Workflow
1. **Start with documentation** (AGENTS.md, README) before code.
2. **Commit after each logical unit**: package, feature, test suite.
3. **Never batch all changes** into a single commit.
4. **Each commit must be independently functional** – build and tests pass.
5. **Verify before committing**: run tests, check linting.

### Commit Points
- After creating a new package with its core types/interfaces
- After implementing a complete feature or command
- After adding tests for a feature (separately from the feature itself)
- After adding or updating documentation
- After fixing a bug (with its test)

### Conventional Commits
Format: `type(scope): description`

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `test`: Adding or correcting tests
- `chore`: Maintenance tasks
- `build`: Build system or external dependencies
- `ci`: CI configuration
- `perf`: Performance improvement
- `security`: Security fix

Rules:
- Lowercase only
- No period at the end
- Max 50 characters for title
- Use imperative mood ("add" not "added")

### Signing Requirements
**All commits must be signed-off and GPG-signed**: `git commit -s -S`
- Sign-off certifies you have the right to submit the code (DCO).
- GPG signature ensures authenticity.

## Security Guidelines

### Critical Security Mindset
**This software handles authentication credentials.** Security is not optional.

### Token Handling
- Tokens must **only** be stored in secure token stores (OS keyring or file-based for testing) – never in plaintext files in production.
- Tokens must not be logged, printed, or included in error messages.
- Token variables should be cleared/overwritten when no longer needed.
- Use constant-time comparison for token validation if applicable.
- **Type separation**:
  - `types.Token` is the internal application model (no JSON annotations, contains only fields used by application logic)
  - `vault.VaultTokenResponse` is for Vault API responses (with JSON annotations)
  - Never use `types.Token` for JSON marshaling/unmarshaling
  - Token metadata (policies, accessor, etc.) is available via `vault.TokenStatus` from lookups, not stored in `types.Token`

### Token Store Security
- **Never fall back to insecure storage** if token store is unavailable.
- Fail loudly if secure storage is not available.
- Validate token store availability before attempting to store secrets.
- The `tokenstore` package provides a `Store` interface with implementations:
  - `KeyringStore`: OS keyring backend (production) - created via `NewKeyringStore()`
  - `FileStore`: File-based backend (testing only, controlled by `PATROL_TEST_KEYRING_DIR` env var) - created via `NewFileStore(dir)`
- **Both stores use constructor functions** for consistent initialization patterns
- **DefaultStore() behavior**:
  - If `PATROL_TEST_KEYRING_DIR` is set, attempts to create a FileStore
  - **Panics if FileStore creation fails** (no fallback to KeyringStore) - this ensures test environments fail fast rather than silently using production keyring
  - Otherwise returns a KeyringStore via `NewKeyringStore()`

### Input Validation
- Validate all user inputs.
- Sanitize data before passing to external commands.
- Prevent command injection in proxy execution.

### Dependency Security
- Minimize dependencies.
- Audit new dependencies before adding.
- Keep dependencies updated for security patches.
- Run `make verify-deps` to ensure integrity.

### Secure Defaults
- TLS verification enabled by default.
- Secure file permissions (0600 for sensitive files, 0700 for directories).
- No sensitive data in environment variable names (only values).

### Security Testing
- Run `make security` on all code.
- Write tests that verify security properties.
- Test error paths to ensure no information leakage.

## Documentation Standards

### Required Documentation
- **README.md**: Installation, quick start, basic usage
- **CONTRIBUTING.md**: Development setup, contribution guidelines, code standards
- **AGENTS.md**: AI agent guidelines (this file)
- **GoDoc comments**: All exported symbols

### GitHub Templates
- Issue templates for bugs, features, security reports
- Pull request template with checklist
- Security policy (SECURITY.md)

### Documentation Quality
- Keep documentation current with code changes.
- Include examples for complex features.
- Document error conditions and edge cases.
- Provide troubleshooting guidance.

## Development Workflow

### Feature Development
1. **Plan**: Understand requirements, identify affected components
2. **Research**: Use subagent to explore existing code patterns
3. **Implement**: Write code following patterns, commit incrementally
4. **Test**: Write unit tests, verify they pass
5. **Integrate**: Write integration tests, verify with real infrastructure
6. **Review**: Use subagent to audit code quality and security
7. **Document**: Update relevant documentation
8. **Verify**: Run full test suite, check coverage, run linters
9. **Commit**: Create final commits with proper messages

### Bug Fixing
1. **Reproduce**: Create a failing test that demonstrates the bug
2. **Investigate**: Understand root cause
3. **Fix**: Make minimal change to fix the issue
4. **Verify**: Ensure test passes, no regressions
5. **Document**: Update docs if behavior changed
6. **Commit**: Reference issue number if applicable

### Code Review Checklist
- [ ] Code follows project conventions
- [ ] Error handling is complete
- [ ] No sensitive data exposure
- [ ] Tests are comprehensive and use mocks (no external dependencies)
- [ ] All functions that interact with external systems accept options for dependency injection
- [ ] Options are passed through call chains (not dropped)
- [ ] Vault operations use executor interfaces (no convenience functions that create executors)
- [ ] TokenManager and other managers accept executors in constructors for testability
- [ ] Tests pass in clean CI environment (verified)
- [ ] Test file names match source file names
- [ ] No wrapper/alias files without value (use original types directly)
- [ ] Core types are in `types` package, not scattered across packages
- [ ] Vault API response types are in `vault` package, not in `types`
- [ ] Documentation is updated
- [ ] No unnecessary complexity
- [ ] Performance is acceptable
- [ ] Security considerations addressed

## Continuous Improvement

### Refactoring Guidelines
- Refactor only with test coverage in place.
- Make small, incremental changes.
- Justify changes in commit messages.
- Ensure no regression in functionality or coverage.

### Technical Debt
- Track technical debt in issues.
- Address debt incrementally, not all at once.
- Prioritize security-related debt.

### Learning and Adaptation
- Update AGENTS.md when new patterns are established.
- Document lessons learned from bugs or issues.
- Continuously improve test coverage.

## Summary

**Quality over speed.** Take the time to do things right:
- Verify everything works before marking complete.
- Use subagents to maintain focus and quality.
- Security is non-negotiable for this project.
- Test coverage proves correctness.
- **Testability is mandatory** - all external dependencies must be mockable.
- **Tests must pass in CI without external tools** - use mocks, never assume binaries exist.
- Documentation enables maintainability.
- Small, verified commits enable collaboration.

Every line of code in this project could affect the security of systems that depend on it. Act accordingly.
