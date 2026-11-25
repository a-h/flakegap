#!/usr/bin/env bash
# Tests a flakegap export of testproject on Ubuntu with a multi-user Nix install.
#
# Runs as root in a container without network access, with /test containing flakegap, export.tar.gz, and this
# script. The first argument is the Nix system, e.g. aarch64-linux.
set -euo pipefail

system="$1"
export_file=/test/export.tar.gz

echo "== Starting the Nix daemon"
nix-daemon &
until [ -S /nix/var/nix/daemon-socket/socket ]; do sleep 0.1; done

echo "== Checking that an untrusted user cannot import the export"
if runuser -u tester -- env HOME=/home/tester /test/flakegap import -import-filename "$export_file" > /tmp/untrusted-import.log 2>&1; then
  echo "FAIL: an untrusted user imported unsigned paths"
  exit 1
fi
grep -m 1 -i "signature" /tmp/untrusted-import.log
echo "OK: the import was refused"

echo "== Importing the export as root"
/test/flakegap import -import-filename "$export_file"

echo "== Extracting the source for the tester user"
tar -xzf "$export_file" -C /home/tester source
chown -R tester:tester /home/tester/source

cat > /tmp/tester.sh <<'EOF'
set -euo pipefail
system="$1"
cd /home/tester/source

echo "== Building each output as the tester user"
for ref in "packages.$system.default" "packages.$system.app-docker-image" "devShells.$system.default"; do
  nix build --no-link ".#$ref"
  echo "OK: built $ref"
done

echo "== Rebuilding after a change to the source"
before="$(nix build --no-link --print-out-paths ".#packages.$system.default")"
echo 'console.log("changed on the airgapped side");' >> node/index.ts
after="$(nix build --no-link --print-out-paths ".#packages.$system.default")"
if [ "$before" = "$after" ]; then
  echo "FAIL: the output did not change after the source changed"
  exit 1
fi
echo "OK: rebuilt $after"

echo "== Starting an interactive development shell"
# The shell computes the marker, so the terminal's echo of the typed input does not match it.
printf '%s\n' 'case $- in *i*) echo "interactive-shell-$((6 * 7))" ;; esac' 'python3 --version' 'exit' \
  | script --quiet --return --command "nix develop .#devShells.$system.default" /dev/null > /tmp/develop.log 2>&1
if ! grep -q "interactive-shell-42" /tmp/develop.log; then
  cat /tmp/develop.log
  echo "FAIL: nix develop did not start an interactive shell"
  exit 1
fi
grep "^Python " /tmp/develop.log
echo "OK: nix develop started an interactive shell"
EOF
runuser -u tester -- env HOME=/home/tester bash /tmp/tester.sh "$system"

echo "PASS"
