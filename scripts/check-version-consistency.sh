#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

go_version=$(sed -n 's/^const Version = "\([^"]*\)"$/\1/p' internal/version/version.go)
webmin_version=$(sed -n 's/^version=//p' webmin/smtp2go-helper/module.info)
debian_version=$(sed -n '1s/^smtp2go-helper (\([^)]*\)).*/\1/p' packaging/debian/changelog)

case "$go_version" in
  ''|*[!0-9.]*|.*|*..*|*.) echo 'invalid Go application version' >&2; exit 1 ;;
esac
if [ "$webmin_version" != "$go_version" ]; then
  echo "version mismatch: Go=$go_version Webmin=$webmin_version" >&2
  exit 1
fi
if [ "$debian_version" != "$go_version-1" ]; then
  echo "version mismatch: Go=$go_version Debian=$debian_version (expected $go_version-1)" >&2
  exit 1
fi
version_regex=$(printf '%s' "$go_version" | sed 's/\./\\./g')
if ! grep -Eq "^## ${version_regex} - [0-9]{4}-[0-9]{2}-[0-9]{2}$" CHANGELOG.md; then
  echo "version mismatch: CHANGELOG.md has no release heading for $go_version" >&2
  exit 1
fi
printf 'version metadata consistent: %s (Debian %s)\n' "$go_version" "$debian_version"
