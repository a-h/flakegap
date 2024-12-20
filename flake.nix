{
  inputs = {
    nix.url = "github:nixos/nix/2.24.10";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    gitignore = {
      url = "github:hercules-ci/gitignore.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    xc = {
      url = "github:joerdav/xc";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nix, nixpkgs, gomod2nix, gitignore, xc }:
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
          overlays = [
            (self: super: {
              nix = nix.packages.${system}.nix;
              xc = xc.packages.${system}.xc;
              gomod2nix = gomod2nix.legacyPackages.${system}.gomod2nix;
            })
          ];
        };
      });

      # Build app.
      app = { name, pkgs, system, version }: gomod2nix.legacyPackages.${system}.buildGoApplication {
        name = name;
        src = gitignore.lib.gitignoreSource ./.;
        go = pkgs.go;
        # Must be added due to bug https://github.com/nix-community/gomod2nix/issues/120
        pwd = ./.;
        subPackages = [ "cmd/${name}" ];
        CGO_ENABLED = 0;
        flags = [
          "-trimpath"
        ];
        ldflags = [
          "-s"
          "-w"
          "-extldflags -static"
          "-X main.version=${version}"
        ];
      };

      # Development tools used.
      devTools = { system, pkgs }: [
        pkgs.crane
        pkgs.docker
        pkgs.gh
        pkgs.git
        pkgs.go
        pkgs.nix
        pkgs.wget
        pkgs.xc
        pkgs.gomod2nix
      ];
    in
    {
      # `nix build` builds the app.
      # `nix build .#docker-image` builds the Docker container.
      packages = forAllSystems ({ system, pkgs }: {
        default = app { name = "flakegap"; pkgs = pkgs; system = system; version = self.sourceInfo.lastModifiedDate; };
        validate = app { name = "validate"; pkgs = pkgs; system = system; version = self.sourceInfo.lastModifiedDate; };
      });
      # `nix develop` provides a shell containing required tools.
      # Run `gomod2nix` to update the `gomod2nix.toml` file if Go dependencies change.
      devShells = forAllSystems ({ system, pkgs }: {
        default = pkgs.mkShell {
          buildInputs = (devTools { system = system; pkgs = pkgs; });
        };
      });
    };
}
