# Security Policy

Patrol is a credential management tool that handles authentication tokens for HashiCorp Vault and OpenBao. Security is our top priority.

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.x.x   | :white_check_mark: |

## Security Design Principles

Patrol follows these security principles:

1. **No Plaintext Storage**: Tokens are NEVER stored in plaintext files
2. **OS Keyring Only**: All credentials use the operating system's secure credential store
3. **Fail Secure**: If secure storage is unavailable, Patrol refuses to store tokens rather than falling back to insecure methods
4. **Minimal Logging**: Tokens and sensitive data are never logged or printed
5. **Principle of Least Privilege**: Patrol only requests the permissions it needs

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

### Preferred Method: GitHub Security Advisories

1. Go to the [Security tab](https://github.com/xabinapal/patrol/security) of this repository
2. Click "Report a vulnerability"
3. Fill out the security advisory form with details about the vulnerability

### Alternative: Private Email

If you cannot use GitHub Security Advisories, email security concerns to:
- **Email**: security@xabinapal.dev (replace with actual security contact)

### What to Include

When reporting a vulnerability, please include:

- **Type of vulnerability** (e.g., token exposure, command injection, privilege escalation)
- **Location** of the vulnerable code (file path, function name)
- **Step-by-step reproduction** instructions
- **Proof of concept** if available
- **Impact assessment** - what an attacker could achieve
- **Suggested fix** if you have one

### Response Timeline

- **Initial Response**: Within 2 business days
- **Status Update**: Within 5 business days
- **Resolution Target**: Within 30 days for critical issues, 90 days for others

### What to Expect

1. **Acknowledgment**: We'll confirm receipt of your report
2. **Investigation**: We'll investigate and validate the vulnerability
3. **Communication**: We'll keep you informed of our progress
4. **Credit**: We'll credit you in the security advisory (unless you prefer anonymity)
5. **Disclosure**: We coordinate disclosure timing with you

## Security Best Practices for Users

### Token Handling

- Regularly rotate your Vault/OpenBao tokens
- Use tokens with appropriate TTLs (not infinite)
- Enable token renewal when possible
- Revoke tokens when no longer needed (`patrol logout`)

### System Security

- Keep Patrol updated to the latest version
- Ensure your OS keyring/credential store is properly secured
- Use strong authentication for your user account
- On Linux, ensure a secure Secret Service provider is running

### Network Security

- Always use TLS (HTTPS) connections to Vault/OpenBao
- Verify TLS certificates (don't disable verification in production)
- Use network segmentation to protect Vault/OpenBao access

## Known Security Considerations

### Keyring Availability

Patrol requires a functioning OS keyring:
- **macOS**: Keychain (built-in)
- **Windows**: Credential Manager (built-in)
- **Linux**: D-Bus Secret Service (GNOME Keyring, KWallet, etc.)

If no keyring is available, Patrol will refuse to store tokens.

### Memory Handling

Go does not provide guaranteed memory clearing. While Patrol attempts to minimize token lifetime in memory, tokens may persist in memory until garbage collected.

### Audit Logging

Patrol does not currently maintain audit logs. For audit requirements, rely on Vault/OpenBao's built-in audit logging.

## Security Audits

This project has not undergone a formal security audit. Contributions to improve security are welcome.

## Acknowledgments

We thank the security researchers who have helped improve Patrol's security:

- (No reports yet)

---

Thank you for helping keep Patrol and its users secure!
