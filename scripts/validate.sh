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
trap 'unlink "$tmp"' EXIT
CGO_ENABLED=0 go build -trimpath -o "$tmp" ./cmd/smtp2go-helper
"$tmp" --version
bash -n scripts/*.sh
bash scripts/check-version-consistency.sh
if command -v perl >/dev/null 2>&1; then
  (cd webmin/smtp2go-helper && for file in *.cgi *.pl; do perl -c "$file"; done)
  perl tests/webmin_security.t
  for file in config.cgi test.cgi queue.cgi reload.cgi; do
    grep -Fq "ui_form_start('$file','post')" "webmin/smtp2go-helper/$file"
    grep -Fq 'sth_require_post();' "webmin/smtp2go-helper/$file"
  done
fi
echo 'validation passed'
