{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
    gitignore = {
      url = "github:hercules-ci/gitignore.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    xc = {
      url = "github:joerdav/xc";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, gitignore, xc }:
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
              xc = xc.packages.${system}.xc;
            })
          ];
        };
      });

      # Build app.
      app = { name, pkgs, version }: pkgs.buildGoModule {
        name = name;
        src = gitignore.lib.gitignoreSource ./.;
        go = pkgs.go;
        subPackages = [ "cmd/${name}" ];
        vendorHash = "sha256-ZBzViO9DbCB05UcLOOqGpQKtrRoMcGxyh65wlNHsL8c=";
        goSum = ./go.sum;
        env = {
          CGO_ENABLED = "0";
        };
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
      devTools = pkgs: [
        pkgs.crane
        pkgs.docker
        pkgs.gh
        pkgs.nixVersions.nix_2_28
        pkgs.git
        pkgs.go
        pkgs.nix
        pkgs.wget
        pkgs.xc
        # Python is only needed for testing flakegap export --export-pypi=true
        pkgs.python312
        pkgs.python312Packages.pip
      ];
    in
    {
      # `nix build` builds the app.
      # `nix build .#docker-image` builds the Docker container.
      packages = forAllSystems ({ system, pkgs }: {
        default = app { name = "flakegap"; pkgs = pkgs; version = self.sourceInfo.lastModifiedDate; };
        validate = app { name = "validate"; pkgs = pkgs; version = self.sourceInfo.lastModifiedDate; };
      });
      # `nix develop` provides a shell containing required tools.
      devShells = forAllSystems ({ system, pkgs }: {
        default = pkgs.mkShell {
          buildInputs = (devTools pkgs);
        };
      });
    };
}
