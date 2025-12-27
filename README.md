# Patrol

**Patrol** is a CLI utility for managing HashiCorp Vault and OpenBao authentication tokens. It provides secure persistent token storage, automatic token renewal, and multi-server profile management.

## Features

- **Secure Token Storage**: Tokens are stored in your OS's native credential store (macOS Keychain, Windows Credential Manager, or Linux Secret Service) - never in plaintext files.
- **CLI Proxy**: Use `patrol` as a drop-in replacement for `vault` or `bao` commands. All commands are transparently forwarded to the underlying CLI.
- **Multi-Profile Support**: Manage connections to multiple Vault/OpenBao servers with easy profile switching.
- **Automatic Token Renewal**: Background daemon automatically renews tokens before they expire.
- **Token Helper Mode**: Can be configured as Vault's token helper for seamless integration.
- **Cross-Platform**: Works on Linux, macOS, and Windows.

## Installation

### From Binary Releases

Download the latest release from the [Releases page](https://github.com/xabinapal/patrol/releases).

```bash
# Linux (amd64)
curl -LO https://github.com/xabinapal/patrol/releases/latest/download/patrol_linux_x86_64.tar.gz
tar xzf patrol_linux_x86_64.tar.gz
sudo mv patrol /usr/local/bin/

# macOS (amd64)
curl -LO https://github.com/xabinapal/patrol/releases/latest/download/patrol_darwin_x86_64.tar.gz
tar xzf patrol_darwin_x86_64.tar.gz
sudo mv patrol /usr/local/bin/

# macOS (Apple Silicon)
curl -LO https://github.com/xabinapal/patrol/releases/latest/download/patrol_darwin_arm64.tar.gz
tar xzf patrol_darwin_arm64.tar.gz
sudo mv patrol /usr/local/bin/
```

### From Source

```bash
git clone https://github.com/xabinapal/patrol.git
cd patrol
make install
```

## Quick Start

### 1. Add a Vault Profile

```bash
# Add a connection profile
patrol profile add dev --address=https://vault.example.com:8200

# For OpenBao
patrol profile add openbao-dev --address=https://openbao.example.com:8200 --type=openbao
```

### 2. Login to Vault

```bash
# Login with any auth method (all Vault auth methods are supported)
patrol login -method=userpass username=admin

# Or token auth
patrol login
```

Your token is now securely stored and will be automatically used for subsequent commands.

### 3. Use Vault Commands

```bash
# All vault commands work through patrol
patrol kv get secret/myapp
patrol kv put secret/myapp password=supersecret
patrol status
```

### 4. Switch Between Profiles

```bash
# Add another profile
patrol profile add prod --address=https://vault.prod.example.com:8200

# Switch to it
patrol use prod

# Login to the new profile
patrol login
```

### 5. Start Auto-Renewal Daemon (Optional)

You can run the daemon in two ways:

**Option 1: Simple background process**
```bash
# Start the daemon in the background
patrol daemon start

# Check daemon status
patrol daemon status

# Stop the daemon
patrol daemon stop
```

**Option 2: System service (recommended)**
```bash
# Install as a system service (starts automatically on login)
patrol daemon install

# Check status
patrol daemon status

# Uninstall the service
patrol daemon uninstall
```

## Configuration

Patrol stores its configuration in the following locations:

- **Linux**: `~/.config/patrol/config.yaml`
- **macOS**: `~/Library/Application Support/patrol/config.yaml` or `~/.config/patrol/config.yaml`
- **Windows**: `%APPDATA%\patrol\config.yaml`

### Example Configuration

```yaml
current: dev
connections:
  - name: dev
    address: https://vault.dev.example.com:8200
    type: vault
  - name: prod
    address: https://vault.prod.example.com:8200
    type: vault
    namespace: admin/team1
  - name: openbao-local
    address: http://localhost:8200
    type: openbao
daemon:
  check_interval: 1m
  renew_threshold: 0.75
  min_renew_ttl: 5m
revoke_on_logout: true
```

### Environment Variables

- `PATROL_CONFIG_DIR`: Override the configuration directory
- `PATROL_PROFILE`: Set the active profile (equivalent to `--profile` flag)

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `patrol login [args]` | Authenticate to Vault and securely store the token |
| `patrol logout [profile]` | Remove stored token and optionally revoke it |
| `patrol status` | Show current authentication status |
| `patrol use <profile>` | Switch to a different profile |

### Profile Management

| Command | Description |
|---------|-------------|
| `patrol profile list` | List all configured profiles |
| `patrol profile add <name>` | Add a new connection profile |
| `patrol profile remove <name>` | Remove a profile |
| `patrol profile show [name]` | Show profile details |

### Daemon Commands

| Command | Description |
|---------|-------------|
| `patrol daemon run` | Run the renewal daemon in foreground |
| `patrol daemon start` | Start the daemon as a background process |
| `patrol daemon stop` | Stop the running daemon |
| `patrol daemon status` | Check daemon and service status |
| `patrol daemon install` | Install as a system service (launchd/systemd/Task Scheduler) |
| `patrol daemon uninstall` | Uninstall the system service |
| `patrol daemon service-start` | Start the installed system service |
| `patrol daemon service-stop` | Stop the installed system service |

### Vault CLI Passthrough

Any command not listed above is passed directly to the underlying Vault/OpenBao CLI:

```bash
patrol kv get secret/foo     # Runs: vault kv get secret/foo
patrol secrets list          # Runs: vault secrets list
patrol operator raft list-peers  # etc.
```

## Token Helper Mode

Patrol can be configured as Vault's token helper. This allows you to use the regular `vault` CLI while Patrol handles token storage.

Add to your `~/.vault` file:

```hcl
token_helper = "/usr/local/bin/patrol"
```

Now when you run `vault login`, the token will be securely stored by Patrol.

## Security

### Token Storage

Patrol uses your operating system's native credential store:

- **macOS**: Keychain
- **Windows**: Credential Manager
- **Linux**: Secret Service (GNOME Keyring, KWallet, etc.)

Tokens are never written to plaintext files. If no secure credential store is available, Patrol will refuse to store tokens and display an error.

### Requirements

- **Linux**: A D-Bus Secret Service provider must be running (e.g., `gnome-keyring`, `kwallet`).
- **macOS**: Keychain access (may prompt for permission on first use).
- **Windows**: No additional setup required.

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting pull requests.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/xabinapal/patrol.git
cd patrol

# Install dependencies
make deps

# Install development tools (golangci-lint, gosec)
make install-dev-tools

# Run tests
make test

# Build
make build
```

### Running Tests

```bash
# Run unit tests
make test

# Run with coverage
make coverage

# Start test infrastructure (Vault and OpenBao in Docker)
make test-infra-up

# Run integration tests (requires test infrastructure)
make test-integration

# Run all tests with infrastructure management
make integration

# Stop test infrastructure
make test-infra-down
```

### Test Infrastructure

Integration tests use Docker to run Vault and OpenBao servers:

```bash
# Start test infrastructure
make test-infra-up

# Services available at:
# - Vault:   http://127.0.0.1:8200 (token: root-token)
# - OpenBao: http://127.0.0.1:8210 (token: root-token)

# Stop test infrastructure
make test-infra-down
```

## License

Patrol is licensed under the MIT License. See [LICENSE](LICENSE) for details.
