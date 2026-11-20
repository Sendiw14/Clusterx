# Security policy

## Reporting a vulnerability

Please report suspected vulnerabilities **privately** through GitHub's private vulnerability reporting for this
repository (the **Security** tab → **Report a vulnerability**). Do not open a public issue for security problems.

Include affected component and version/commit, reproduction steps, and impact. We aim to acknowledge reports
promptly; there is no formal response-time commitment before 1.0.

## Supported versions

CLUSTERX is pre-1.0 and unreleased. Only the current `main` branch receives fixes.

## Scope notes

- Handlers are trusted, precompiled code. Running untrusted code, shell commands or user-supplied URLs is out
  of scope and not a supported configuration.
- The public `/demo` is fixture-only and must never reach a live control plane; a demo that does is a bug.
- Threat model and verified controls: [`docs/security/threat-model.md`](docs/security/threat-model.md),
  [`docs/security/verification.md`](docs/security/verification.md).
