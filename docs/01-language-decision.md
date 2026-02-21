# Language Decision: RESOLVED — Go

## Decision: Go

**Date:** February 2026

Go was selected as the implementation language for the full stack. The design document (`worldsim-design.md`) was originally written with Rust examples, but all concepts translate directly to Go.

## Rationale

- **Development velocity**: Go's built-in HTTP, JSON, and concurrency primitives eliminate the plumbing burden that would dominate C or even Rust development time
- **Memory safety**: Critical for a 24/7 long-running server — no manual memory management bugs crashing the world at 3am
- **Systems feel**: Compiled, statically typed, explicit error handling, pointers — satisfies the "close to C" preference without the footguns
- **Simple concurrency**: Goroutines for parallel agent processing, API serving, and external API calls
- **Fast compilation**: Quick iteration cycles during development

## Translation Notes (Rust → Go)

| Rust Concept | Go Equivalent |
|-------------|---------------|
| `struct` | `struct` (identical) |
| `trait` (e.g., `ConjugateField`) | `interface` |
| `enum` with variants | `const` iota + type, or interface with implementations |
| `Vec<T>` | `[]T` (slice) |
| `Option<T>` | pointer `*T` or zero value |
| `Result<T, E>` | `(T, error)` return pattern |
| `axum` / `tokio` | `net/http` + goroutines (built-in) |
| `sqlx` | `database/sql` + `modernc.org/sqlite` |
| `serde` / `serde_json` | `encoding/json` (built-in) |
| `noise` crate | `github.com/ojrac/opensimplex-go` or similar |
| `reqwest` | `net/http` (built-in) |
| `tracing` | `log/slog` (built-in since Go 1.21) |
