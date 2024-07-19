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
tar -xzf nix-export.tar.gz
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

## Tasks

### gomod2nix-update

```bash
gomod2nix
```

### build

```bash
nix build
```

### develop

```bash
nix develop
```

### docker-build

```bash
docker build -t ghcr.io/a-h/flakegap:latest .
```

### docker-run

```bash
docker run -v $PWD:/code:Z -v $PWD/nix-export:/nix-export ghcr.io/a-h/flakegap:latest
```
