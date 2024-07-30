# github.com/a-h/flakegap

Export a Flake's requirements so that they can be transferred across an airgap.

## How does it work?

- Mounts the Flake code into a fresh Docker container, and runs all builds.
- Exports the Nix store from the container to a tarball.
- Validates the build by importing the tarball, and running the build in a Docker container with no network access.

## Usage

### Export

Export the current directory's requirements to a `nix-export.tar.gz` file.

```bash
flakegap export
```

Import the `nix-export.tar.gz` file into the target environment along with the Flake code.

```bash
mkdir nix-export
tar -xzf nix-export.tar.gz --directory ./nix-export
nix copy --all --offline --impure --no-check-sigs --from file://$PWD/nix-export/
```

Use the flake as normal.

```bash
cd ./flake-source-code
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

To run the first part of the export, create a `nix-export.tar.gz` by running the `runtim export` command inside a Docker container that has network access.

You can specify `runtime export -substituter=http://localhost:41805?trusted=1` and run `flakegap serve` to run a local binary cache to speed things up by using your local Nix cache inside the Docker container.

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

### gomod2nix-update

```bash
gomod2nix
```

### build

```bash
nix build
```

### build-runtime

```bash
nix build .#runtime
```

### develop

```bash
nix develop
```

### docker-build

```bash
docker build -t ghcr.io/a-h/flakegap:latest .
docker image tag ghcr.io/a-h/flakegap:latest ghcr.io/a-h/flakegap:local
```

### docker-run

```bash
docker run -v $PWD:/code:Z -v $PWD/nix-export:/nix-export ghcr.io/a-h/flakegap:latest
```

### serve-run

```bash
nix run .#default -- serve
```

### test

```bash
echo "Exporting Flake requirements..."
nix run .#default -- export -image ghcr.io/a-h/flakegap:local
echo "Validating Flake requirements..."
nix run .#default -- validate -image ghcr.io/a-h/flakegap:local
```
