#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
go mod tidy
gofmt -w cmd internal
go test ./...
go vet ./...
if command -v gcc >/dev/null 2>&1; then
  CGO_ENABLED=1 go test -race ./...
else
  echo 'race detector skipped: cgo compiler unavailable'
fi
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
CGO_ENABLED=0 go build -trimpath -o "$tmp" ./cmd/smtp2go-helper
"$tmp" --version
bash -n scripts/*.sh
if command -v perl >/dev/null 2>&1; then
  (cd webmin/smtp2go-helper && for file in *.cgi *.pl; do perl -c "$file"; done)
fi
echo 'validation passed'
