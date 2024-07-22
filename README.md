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
