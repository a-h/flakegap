{
  inputs = {
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

  outputs = { self, nixpkgs, gomod2nix, gitignore, xc }:
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

      # Build app.
      app = { name, pkgs, system }: gomod2nix.legacyPackages.${system}.buildGoApplication {
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
        ];
      };

      # Development tools used.
      devTools = { system, pkgs }: [
        pkgs.crane
        pkgs.gh
        pkgs.git
        pkgs.go
        xc.packages.${system}.xc
        gomod2nix.legacyPackages.${system}.gomod2nix
      ];

      name = "flakegap";
    in
    {
      # `nix build` builds the app.
      # `nix build .#docker-image` builds the Docker container.
      packages = forAllSystems ({ system, pkgs }: {
        default = app { name = name; pkgs = pkgs; system = system; };
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
