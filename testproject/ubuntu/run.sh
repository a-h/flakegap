#!/usr/bin/env bash
# Exports testproject, then tests the export on Ubuntu with a multi-user Nix install and no network access.
#
# Set UBUNTU_VERSION to test a different Ubuntu release, e.g. UBUNTU_VERSION=24.04. Requires Docker, and Nix that can
# build for Linux. The working directory is under $HOME, because Docker
# Desktop and colima share $HOME with their virtual machine.
set -euo pipefail

repo="$(cd "$(dirname "$0")/../.." && pwd)"
ubuntu_version="${UBUNTU_VERSION:-26.04}"

case "$(docker info --format '{{.Architecture}}')" in
  aarch64 | arm64) goarch=arm64 nixarch=aarch64 ;;
  x86_64 | amd64) goarch=amd64 nixarch=x86_64 ;;
  *) echo "unsupported Docker architecture" >&2; exit 1 ;;
esac

mkdir -p "$HOME/.cache"
work="$(mktemp -d "$HOME/.cache/flakegap-ubuntu-test.XXXXXX")"
trap 'rm -rf "$work"' EXIT

echo "== Building flakegap for this machine and for linux/$goarch"
(cd "$repo" && go build -o "$work/flakegap-host" ./cmd/flakegap)
(cd "$repo" && CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build -o "$work/flakegap" ./cmd/flakegap)
cp "$repo/testproject/ubuntu/test.sh" "$work/test.sh"

echo "== Exporting testproject for $nixarch-linux"
# Copy the files that git tracks or does not ignore, which leaves out .venv, node_modules, and result.
(cd "$repo" && git ls-files --cached --others --exclude-standard -- testproject | grep -v '^testproject/ubuntu/' \
  | tar -cf - -T -) | tar -xf - -C "$work"
(cd "$work/testproject" && "$work/flakegap-host" export --use-filesystem-copy --architecture "$nixarch" \
  --platform linux --temporary-path "$work" --export-filename "$work/export.tar.gz")

echo "== Building the Ubuntu $ubuntu_version image"
docker build --build-arg "UBUNTU_VERSION=$ubuntu_version" --tag "flakegap-ubuntu-test:$ubuntu_version" "$repo/testproject/ubuntu"

echo "== Testing the export without network access"
docker run --rm --network=none --volume "$work:/test" "flakegap-ubuntu-test:$ubuntu_version" bash /test/test.sh "$nixarch-linux"
