# Changelog

All notable changes to `project-tap` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
*(No new changes yet)*

## [0.0.1] - 2026-08-11

### Added
- **Core Domains:** Implemented modular handlers, services, and repositories for the `admin`, `merchant`, and `user` domains.
- **Authentication:** Built the authentication repository and login handler.
- **Middleware:** Introduced middleware for route authentication and rate limiting to secure endpoints.
- **Integrations:** Added service connections for MQTT, R2 object storage, Xendit payout APIs, and SMTP.
- **CI/CD Pipeline:** Created a CI workflow to enforce PR branch flow rules and automatically execute Go tests on push/pull requests.
- **Utilities:** Added JSON response helpers to the general `pkg` directory to standardize API outputs.

### Changed
- **Auth & Pkg Updates:** Refactored the existing authentication domain and general package utilities to support the new multi-domain architecture and third-party integrations.

[unreleased]: https://github.com/devzeeh/project-tap/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/devzeeh/project-tap/releases/tag/v0.0.1
