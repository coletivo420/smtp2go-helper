# Contributing

Use Go 1.24 or newer. Run `gofmt -w cmd internal`, `go test ./...`, `go vet ./...`, and `scripts/validate.sh` before opening a pull request. Do not use real SMTP2GO credentials in tests or CI. Add regression tests and update `docs/AI-DEVELOPMENT.md` when changing an invariant.

The GPL-3.0-or-later project may import dependencies under their own licenses. Do not copy or vendor SMTP2GO SDK source without an explicit reason and license review.
