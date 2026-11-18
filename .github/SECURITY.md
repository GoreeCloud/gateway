# GoreeCloud Gateway security policy

GoreeCloud Gateway is a development-stage, Caddy-powered reverse proxy and
loopback-only management console. No current Gateway development release is
certified or authorized for production deployment. The repository does not
operate, accept incident reports for, or provide access to any private host.

## Report a vulnerability privately

Use [GitHub private vulnerability reporting](https://github.com/GoreeCloud/gateway/security/advisories/new)
for suspected vulnerabilities in GoreeCloud Gateway source, routes,
management authentication, storage or release artifacts. This feature is
enabled for the repository. Do not post exploit details, credentials,
production host identifiers, certificate files or private backup information
in public issues, pull requests or logs.

Include the affected source commit or tagged build, the suspected impact,
and minimal **synthetic** reproduction steps when safely possible. If the
issue originates in upstream Caddy or a separately maintained dependency,
also follow that upstream maintainer's security reporting process.

## Supported release status

- **Latest protected main:** receives active development changes and required
  CI security checks. A passing check is not a production acceptance or
  vulnerability-free guarantee.
- **v0.1.0-alpha.4:** development-only release with patched Go dependency
  versions; verify exact source, build provenance and its SHA-256 checksums.
- **v0.1.0-alpha.1 through alpha.3:** superseded development releases with
  published security notices for a vulnerable compression dependency.
  Do not deploy these builds.

There is no production support promise, SLA, confidential data collection
request or automated production patching in this policy. Changes follow
reviewable protected pull requests and verification. Maintainers may use
GitHub advisories to coordinate disclosure where appropriate.
