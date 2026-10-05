# Development

Use Go 1.24+ and standard Go tooling. `go mod tidy`, `gofmt`, `go test ./...`, `go vet ./...`, `go test -race ./...`, and `scripts/validate.sh` are the local validation path. HTTP tests use `httptest`; no real API credentials or network send are required.

The SMTP2GO SDK dependency is MIT licensed and remains an upstream module. This project is GPL-3.0-or-later. Importing the SDK types does not copy its source into this repository. Review licenses on any new dependency before adding it. `golang.org/x/net` is used only for standards-aware charset conversion.
