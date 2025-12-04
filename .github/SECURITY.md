# Security Policy

## Supported Versions

We actively support the following versions with security updates:

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| develop | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you believe you have found a security vulnerability in the Kartoza Timesheet App, please report it to us as described below.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please send an email to security@kartoza.com with:

- A description of the vulnerability
- Steps to reproduce the issue
- Affected versions
- Any possible mitigations you've identified

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your report within 48 hours
- **Investigation**: We will investigate and validate the reported vulnerability
- **Communication**: We will keep you informed about our progress
- **Resolution**: We will work to address confirmed vulnerabilities promptly

### Responsible Disclosure

- We ask that you give us reasonable time to investigate and address the issue
- Please avoid accessing, modifying, or deleting data that doesn't belong to you
- Please don't perform actions that could negatively impact our users or services
- We appreciate your efforts to responsibly disclose your findings

### Bug Bounty

Currently, we do not offer a formal bug bounty program. However, we deeply appreciate security researchers who help us keep our users safe and will acknowledge your contribution in our security advisories (with your permission).

## Security Features

### Current Security Measures

- **Automated Security Scanning**: Trivy vulnerability scanner in CI/CD
- **Secret Detection**: Pre-commit hooks to prevent secret commits
- **Dependency Scanning**: Regular checks for vulnerable dependencies
- **SARIF Integration**: Security findings integrated with GitHub Security tab

### Planned Security Features

- [ ] Authentication and authorization framework
- [ ] Input validation and sanitization
- [ ] Rate limiting and DDoS protection
- [ ] Audit logging for security events
- [ ] Encryption for sensitive data
- [ ] Secure session management
- [ ] HTTPS enforcement
- [ ] Security headers implementation

## Security Best Practices

### For Contributors

- Never commit secrets, API keys, or sensitive information
- Use environment variables for configuration
- Follow secure coding practices
- Run security scans before submitting PRs
- Keep dependencies up to date

### For Users

- Use strong, unique passwords
- Enable two-factor authentication when available
- Keep your installation up to date
- Report suspicious activity
- Follow the principle of least privilege

## Security Contacts

- **Security Issues**: security@kartoza.com
- **General Security Questions**: tim@kartoza.com
- **PGP Key**: Available upon request

## Acknowledgments

We would like to thank the following individuals for their contributions to our security:

- *No reports yet - be the first!*

---

*Last updated: 2025-12-04*