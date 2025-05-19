# github.com/a-h/flakegap

Export a Flake's requirements so that they can be transferred across an airgap.

## How does it work?

- Invokes various Nix commands to run all builds, and copy all derivations and the realised paths of all derivations required to build the outputs.
- Exports the Nix paths to a tarball.
- Can validates the build by importing the tarball, and running the build in a Docker container with no network access.

## Usage

### Export

Export the current directory's requirements to a `nix-export.tar.gz` file.

```bash
flakegap export
```

Import the `nix-export.tar.gz` file into the target environment along with the Flake code.

```bash
flakegap import
```

If you don't have `flakegap` on the target machine, you can use the following commands:

```bash
mkdir nix-export
tar -xzf nix-export.tar.gz --directory ./nix-export
nix copy --all --offline --impure --no-check-sigs --from file://$PWD/nix-export/nix-store/
```

At the `nix copy` operation point, you may get a "path not valid" error. This is due to a bug in Nix - https://github.com/NixOS/nix/issues/9052

You can work around it by importing the paths one by one.

```bash
while read -r p; do
  nix copy $p --from file://$PWD/nix-export/nix-store --no-check-sigs;
done < ./nix-export/nix-export-x86_64-linux.txt
```

If you see the error message: "Cannot add path because it lacks a signature by a trusted key", despite using `--no-check-sigs`, it's likely because you're not a trusted user of Nix. To add yourself, configure `trusted-users` in the `/etc/nix/nix.conf` file, e.g.:

```
trusted-users = <your-username> @wheel
```

Use the flake as normal.

```bash
cd ./nix-export/code
nix develop
nix build
```

### Validate

Check that the Flake code can be built in an airgapped environment.

```bash
flakegap validate
```

### Manual validation

If you want to test it manually, you can run the Docker container interactively with no network access.

To run the first part of the export, create a `nix-export.tar.gz` by running the `runtime export` command inside a Docker container that has network access.

```bash
docker run -it --rm --network host -v $PWD:/code:Z -v $PWD/nix-export:/nix-export --entrypoint=/bin/bash ghcr.io/a-h/flakegap:latest
runtime export
```

This will create a `./nix-export/nix-export.tar.gz` file. Then, you can run the validation, to validate the build.

```bash
mkdir nix-export
tar -xzf nix-export.tar.gz --directory ./nix-export
docker run -it --rm --network none -v $PWD:/code:Z -v $PWD/nix-export:/nix-export --entrypoint=/bin/bash ghcr.io/a-h/flakegap:latest
```

Inside the container, the default working directory is the `/code` directory.

You can run the import:

```bash
nix copy --all --offline --impure --no-check-sigs --from file:///nix-export/
```

And run the build:

```bash
nix build
```

## Installation

### Nix shell (source)

```bash
nix shell github:a-h/flakegap
```

### Nix flake (source)

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flakegap = {
      url = "github:a-h/flakegap";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };
  outputs = { self, nixpkgs, flakegap }:
    let
      allSystems = [
        "x86_64-linux" # 64-bit Intel/AMD Linux
        "aarch64-linux" # 64-bit ARM Linux
        "x86_64-darwin" # 64-bit Intel macOS
        "aarch64-darwin" # 64-bit ARM macOS
      ];

      forAllSystems = f: nixpkgs.lib.genAttrs allSystems (system: f {
        system = system;
        pkgs = import nixpkgs {
          inherit system;
        };
      });

      devShellTools = { system, pkgs }: with pkgs; [
        flakegap.packages.${system}.default
        nmap
      ];
    in
    {
      devShells = forAllSystems ({ system, pkgs }: {
        default = pkgs.mkShell {
          buildInputs = (devShellTools { system = system; pkgs = pkgs; });
        };
      });
    };
}
```

## Nix flake (Flakehub)

Add it to your project with Flakehub's CLI:

```bash
nix run "https://flakehub.com/f/DeterminateSystems/fh/*.tar.gz" add "a-h/flakegap"
```

You can then use the Flake input, as shown here:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flakegap = {
      url = "https://flakehub.com/f/a-h/flakegap/*.tar.gz";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };
  outputs = { self, nixpkgs, flakegap }:
    let
      allSystems = [
        "x86_64-linux" # 64-bit Intel/AMD Linux
        "aarch64-linux" # 64-bit ARM Linux
        "x86_64-darwin" # 64-bit Intel macOS
        "aarch64-darwin" # 64-bit ARM macOS
      ];

      forAllSystems = f: nixpkgs.lib.genAttrs allSystems (system: f {
        system = system;
        pkgs = import nixpkgs {
          inherit system;
        };
      });

      devShellTools = { system, pkgs }: with pkgs; [
        flakegap.packages.${system}.default
        nmap
      ];
    in
    {
      devShells = forAllSystems ({ system, pkgs }: {
        default = pkgs.mkShell {
          buildInputs = (devShellTools { system = system; pkgs = pkgs; });
        };
      });
    };
}
```

## Tasks

### build

```bash
nix build
```

### build-validate

```bash
nix build .#validate
```

### develop

```bash
nix develop
```

### deps-update

```bash
./deps-update.sh
```

### docker-build-local-all

This requires the containerd setting to be enabled, since by default the local Docker store can't store multiple architectures.

```bash
docker buildx build --load --platform linux/amd64,linux/arm64 -t ghcr.io/a-h/flakegap:local .
```

### docker-build-local-amd64

```bash
docker buildx build --load --platform linux/amd64 -t ghcr.io/a-h/flakegap:local .
```

### docker-build-local-arm64

```bash
docker buildx build --load --platform linux/arm64 -t ghcr.io/a-h/flakegap:local .
```

### docker-run

dir: testproject
interactive: true

```bash
docker run -v $PWD:/code:Z -v $PWD/nix-export:/nix-export ghcr.io/a-h/flakegap:latest
```

### docker-run-interactive

dir: testproject
interactive: true

```bash
docker run -it --rm --entrypoint=/bin/bash -v $PWD:/code:Z -v $PWD/nix-export:/nix-export ghcr.io/a-h/flakegap:latest
```

### docker-run-interactive-local

dir: testproject
interactive: true

```bash
docker run -it --rm --entrypoint=/bin/bash -v $PWD:/code:Z -v $PWD/nix-export:/nix-export ghcr.io/a-h/flakegap:local
```

### docker-run-interactive-local-airgapped

After running an export, you can test whether it's possible to build the results by runniing `flakegap import` and `nix build .#app-docker-image` etc.

To see the graph of dependencies, build the output, and you can see the graph with: `nix-store -q --graph `nix path-info --derivation .#app-docker-image` > output.dot`

dir: testproject
interactive: true

```bash
docker run -it --rm --network=none --entrypoint=/bin/bash -v $PWD:/code:Z -v $PWD/nix-export:/nix-export ghcr.io/a-h/flakegap:local
```

### test

dir: testproject
interactive: true

```bash
echo "Testing Nix build..."
echo "Exporting Flake requirements..."
nix run .#default -- export
echo "Validating Flake requirements..."
nix run .#default -- validate -image ghcr.io/a-h/flakegap:local
```

### test-local-export

dir: testproject
interactive: true

```bash
echo "Testing Go build without Nix..."
echo "Exporting Flake requirements..."
go run ../cmd/flakegap export
```

### test-local-validate

dir: testproject
interactive: true

```bash
echo "Validating Flake requirements..."
go run ../cmd/flakegap validate -image ghcr.io/a-h/flakegap:local
```
