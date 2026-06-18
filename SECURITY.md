# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do NOT** open a public GitHub issue
2. Email security concerns to: https://github.com/sachn-cs/promptsheon/issues
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 1 week
- **Fix timeline**: Depends on severity, typically within 2 weeks

## Security Measures

- Authentication is enabled by default
- All SQL queries use parameterized statements
- SHA-256 for hashing, AES-256-GCM for encryption
- Shell command sandboxing with allowlist/blocklist
- Rate limiting on all endpoints
- Request body size limits (10MB)
- Security headers (X-Content-Type-Options, X-Frame-Options, etc.)
- Audit trail with hash-chain integrity
