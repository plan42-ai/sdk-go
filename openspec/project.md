# Project Context

## Purpose
Go SDK for the Plan42 API, including client bindings and CLI helpers (e.g., `p42-ctl`) used across services and tools.

## Tech Stack
- Go 1.25
- HTTP client code generated/maintained under `p42` and `internal`
- CLI tooling under `cmd` using kong

## Project Conventions

### Code Style
Use gofmt and golangci-lint (`make lint`). Keep API types and method names aligned with server definitions in `API.md`. Prefer context-aware methods and explicit error handling.

### Architecture Patterns
SDK code encapsulates request/response models and client helpers in packages under `p42` and `internal`. CLI utilities build atop the same client for diagnostics and operations.

### Testing Strategy
`go test ./...` covers client helpers and data models. Add regression tests for API contract changes and CLI behaviors when updating endpoints.

### Git Workflow
Feature branches with PR review. Tag releases via `make tag` when publishing new SDK versions; ensure version bumps accompany API changes.

## Domain Context
SDK mirrors the Plan42 API contract described in `API.md`. Consumers rely on stable structs and error types, so breaking changes should be coordinated with backend releases.

## Important Constraints
- Maintain backward compatibility where possible; version appropriately when breaking changes occur.
- Avoid coupling SDK to specific runtime environments beyond standard Go HTTP client configuration.

## External Dependencies
- Plan42 API endpoints consumed by the client
