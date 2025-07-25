# Security Guide for OpenMorph

This document outlines the security practices, tools, and procedures implemented in OpenMorph to ensure a secure and production-ready CLI tool.

## Security Overview

OpenMorph implements a comprehensive security strategy covering:

- **Vulnerability Scanning**: Automated detection of known vulnerabilities
- **Dependency Security**: Monitoring and reviewing third-party dependencies
- **Code Analysis**: Static security analysis of source code
- **Supply Chain Security**: Ensuring build and release process integrity

## Security Tools and Scanning

### 1. Govulncheck (Primary Vulnerability Scanner)

We use the official [golang/govulncheck-action](https://github.com/golang/govulncheck-action) for vulnerability scanning:

```yaml
- name: Run govulncheck
  uses: golang/govulncheck-action@v1
  with:
    go-version-input: go.mod
    go-package: ./...
    cache: true
```

**Local Usage:**

```bash
# Run vulnerability scan
make security

# Run with JSON output for detailed analysis
make security-json
```

### 2. CodeQL Security Analysis

Advanced static analysis using GitHub's CodeQL:

- Analyzes Go code for security vulnerabilities
- Runs on every push and pull request
- Results uploaded to GitHub Security tab

### 3. Dependency Review

For pull requests, we run dependency review to:

- Check for known vulnerabilities in new dependencies
- Verify license compatibility
- Monitor for suspicious dependency changes

Allowed licenses:

- MIT
- Apache-2.0
- BSD-2-Clause
- BSD-3-Clause
- ISC

### 4. Trivy Vulnerability Scanner

Additional filesystem scanning using Aqua Security's Trivy:

- Scans for vulnerabilities in dependencies
- Results uploaded as SARIF to GitHub Security tab

### 5. Nancy Dependency Scanner

Sonatype's Nancy scanner for additional dependency vulnerability detection.

## Security Workflows

### Automated Security Scanning

Security scans run automatically:

- **On every push** to main branch
- **On every pull request**
- **Daily at 2 AM UTC** (scheduled scan)
- **Manual trigger** available via GitHub Actions

### Security Report Artifacts

Security scan results are preserved as artifacts:

- JSON reports stored for 30 days
- SARIF files uploaded to GitHub Security tab
- Detailed vulnerability information available

## Security Best Practices

### 1. Go Version Management

- Always use the latest patch version of Go
- Monitor Go security advisories
- Update `go.mod` when security patches are released

**Current Go Version:** 1.24.4 (fixes GO-2025-3750)

### 2. Dependency Management

```bash
# Check for dependency updates
go list -u -m all

# Verify module checksums
go mod verify

# Clean up unused dependencies
go mod tidy
```

### 3. Secure File Operations

The codebase follows secure file handling practices:

- Proper error handling for file operations
- Secure temporary file creation
- Cleanup of temporary resources

### 4. Input Validation

- All user inputs are validated
- Configuration files are properly parsed and validated
- Error messages don't leak sensitive information

## Security Configuration

### Environment Variables

Sensitive configuration should use environment variables:

```bash
# Example: Custom config location
export OPENMORPH_CONFIG="/secure/path/config.yaml"
```

### File Permissions

Recommended file permissions:

- Configuration files: `644` (readable by owner/group)
- Executable: `755` (executable by all, writable by owner)
- Private keys/secrets: `600` (readable by owner only)

## Vulnerability Response Process

### 1. Detection

- Automated scans detect vulnerabilities
- GitHub Security Advisories notify of issues
- Community reports via GitHub Issues

### 2. Assessment

- Evaluate impact on OpenMorph functionality
- Determine affected versions
- Assess severity level

### 3. Remediation

- Update dependencies to patched versions
- Apply code fixes if needed
- Update Go version if standard library is affected

### 4. Release

- Create patch release with security fixes
- Update security documentation
- Notify users of security updates

## Security Testing

### Manual Security Testing

```bash
# Run complete security suite
make security
make test
make lint

# Check for hardcoded secrets
grep -r "password\|secret\|key\|token" --exclude-dir=.git .

# Verify file permissions
find . -type f -executable | xargs ls -la
```

### Security Test Cases

The test suite includes security-focused tests:

- Input validation testing
- Error handling verification
- Temporary file cleanup validation
- Configuration parsing security

## Security Contacts

### Reporting Security Issues

**Please do not report security vulnerabilities through public GitHub issues.**

For security issues:

1. Create a private security advisory on GitHub
2. Email: [security contact if available]
3. Include detailed description and reproduction steps

### Security Team Response

- **Initial Response:** Within 24 hours
- **Assessment:** Within 72 hours
- **Fix Timeline:** Based on severity
  - Critical: Within 24-48 hours
  - High: Within 1 week
  - Medium: Within 2 weeks
  - Low: Next regular release

## Security Checklist for Development

### Before Committing

- [ ] Run `make security` locally
- [ ] Run `make test` to ensure all tests pass
- [ ] Run `make lint` for code quality
- [ ] Review changes for sensitive information
- [ ] Update dependencies if security patches available

### Before Releasing

- [ ] Run full security scan
- [ ] Verify all dependencies are up to date
- [ ] Check for any outstanding security advisories
- [ ] Test with security-focused scenarios
- [ ] Update security documentation if needed

## Security Monitoring

### Continuous Monitoring

- Daily automated vulnerability scans
- GitHub Dependabot alerts enabled
- Security advisory notifications
- Go security mailing list monitoring

### Metrics and Reporting

Security metrics tracked:

- Number of vulnerabilities found/fixed
- Time to patch critical vulnerabilities
- Dependency update frequency
- Security scan coverage

## Additional Resources

### External Security Tools

Recommended additional tools for development:

```bash
# Install security linters
go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

# Run additional security checks
gosec ./...
staticcheck ./...
```

### Security References

- [Go Security Policy](https://golang.org/security)
- [OWASP Go Secure Coding Practices](https://owasp.org/www-project-go-secure-coding-practices-guide/)
- [CIS Go Benchmarks](https://www.cisecurity.org/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)

## Version History

- **v1.0.0**: Initial security implementation
- **Current**: Enhanced with official govulncheck-action and comprehensive scanning

---

**Last Updated:** July 2025
**Security Version:** 1.0
**Next Review:** Next major release
