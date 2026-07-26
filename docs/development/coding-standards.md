# Coding Standards & Guidelines

This document outlines the coding standards, patterns, and style guidelines for developers contributing to the AEGIS codebase.

## Go Code Guidelines

### 1. Style & Formatting
* **Standard Formatting**: All Go code must be formatted using `gofmt` and checked with `go vet`.
* **Linting**: We enforce strict linting using `golangci-lint` (as configured in CI quality gates).
* **Vulnerability Scanning**: Run `govulncheck` regularly to catch CVEs in import modules.

### 2. Error Handling
* Wrap errors when returning from infrastructure layers to add context: `fmt.Errorf("reading database pool: %w", err)`.
* Use package `pkg/apperrors` to return RFC-7807 structured HTTP error models.
* Avoid panic-based control flows. Recover from panics only in web server middlewares.

### 3. Concurrency
* Always manage timeouts using `context.Context` (propagation to DB and external calls).
* Guard shared resource mutations with sync primitives (e.g. `sync.Mutex` or `sync.RWMutex`).
* Avoid goroutine leaks; ensure channels are closed properly.

---

## Frontend (React/TypeScript) Guidelines

### 1. Types & Safety
* **Strict TypeScript**: Do not use `any` type annotations. Explicitly type component props and state.
* **Vite + React**: Leverage Vite's fast HMR. Build must compile without warning flags under `tsc`.

### 2. Styling
* Follow the theme variables defined in `/frontend/src/index.css`.
* Do not inject inline hardcoded colors or sizing values. Keep styles responsive.
