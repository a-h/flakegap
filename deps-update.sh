#!/usr/bin/env bash
set -eux
wget https://github.com/DeterminateSystems/nix-installer/releases/download/v3.5.2/nix-installer-aarch64-linux
wget https://github.com/DeterminateSystems/nix-installer/releases/download/v3.5.2/nix-installer-x86_64-linux
sha256sum nix-installer-*-linux > deps.sha256sum
rm nix-installer-*-linux
