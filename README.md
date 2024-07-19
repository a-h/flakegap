# github.com/a-h/flake-templates/go

## Tasks

### gomod2nix-update

```bash
gomod2nix
```

### build

```bash
nix build
```

### run

```bash
nix run
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
