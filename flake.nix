{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    gitignore = {
      url = "github:hercules-ci/gitignore.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    xc = {
      url = "github:joerdav/xc";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    version = {
      url = "github:a-h/version/0.0.12";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      gitignore,
      xc,
      version,
    }:
    let
      allSystems = [
        "x86_64-linux" # 64-bit Intel/AMD Linux
        "aarch64-linux" # 64-bit ARM Linux
        "x86_64-darwin" # 64-bit Intel macOS
        "aarch64-darwin" # 64-bit ARM macOS
      ];

      forAllSystems =
        f:
        nixpkgs.lib.genAttrs allSystems (
          system:
          f {
            system = system;
            pkgs = import nixpkgs {
              inherit system;
              overlays = [
                (self: super: {
                  xc = xc.packages.${system}.xc;
                  version = version.packages.${system}.default;
                })
              ];
            };
          }
        );

      nixConf = ''
        build-users-group =
        experimental-features = nix-command flakes
        filter-syscalls = false
        system-features = nixos-test benchmark big-parallel kvm
      '';

      # Build Docker containers.
      dockerImage =
        { pkgs }:
        let
          validateApp = app {
            name = "validate";
            pkgs = pkgs;
            version = versionFileContents;
          };
          flakegapApp = app {
            name = "flakegap";
            pkgs = pkgs;
            version = versionFileContents;
          };
        in
        pkgs.dockerTools.buildLayeredImage {
          name = "ghcr.io/a-h/flakegap";
          tag = "latest";
          contents = [
            pkgs.bashInteractive
            pkgs.cacert
            pkgs.coreutils
            pkgs.curl
            pkgs.dockerTools.caCertificates
            pkgs.git
            pkgs.nix
            pkgs.xz
            validateApp
            flakegapApp
          ];
          enableFakechroot = true;
          fakeRootCommands = ''
            mkdir -p /etc/nix
            printf '%s' '${nixConf}' > /etc/nix/nix.conf
            mkdir -p /nix/var/nix/db
            mkdir -p /nix/var/nix/gcroots/auto
            mkdir -p /nix/var/nix/profiles/per-user/root
            mkdir -p /nix/var/nix/temproots
            mkdir -p /root
            mkdir -p /code
            mkdir -p /nix-export
            mkdir -p /tmp
            chmod 1777 /tmp
            mkdir -p /usr/local/bin
            ln -s ${validateApp}/bin/validate /usr/local/bin/validate
            ln -s ${flakegapApp}/bin/flakegap /usr/local/bin/flakegap
          '';
          config = {
            Entrypoint = [ "/usr/local/bin/validate" ];
            WorkingDir = "/code";
            Env = [
              "NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-bundle.crt"
              "HOME=/root"
              "USER=root"
            ];
          };
        };

      # Build app.
      app =
        {
          name,
          pkgs,
          version,
        }:
        pkgs.buildGoModule {
          name = name;
          src = gitignore.lib.gitignoreSource ./.;
          go = pkgs.go;
          subPackages = [ "cmd/${name}" ];
          vendorHash = "sha256-N8CGnrKa//15Yo3rFcgs1gs+O12fjMdUnRjF/oMov8M=";
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
        pkgs.git
        pkgs.go
        pkgs.nix
        pkgs.nixVersions.nix_2_28
        pkgs.version
        pkgs.wget
        pkgs.xc
        # Python is only needed for testing flakegap export --export-pypi=true
        pkgs.python312
        pkgs.python312Packages.pip
      ];

      versionFileContents = builtins.readFile ./.version;
    in
    {
      # `nix build` builds the app.
      # `nix build .#docker-image` builds the Docker container (Linux only).
      packages = forAllSystems (
        { system, pkgs }:
        {
          default = app {
            name = "flakegap";
            pkgs = pkgs;
            version = versionFileContents;
          };
          validate = app {
            name = "validate";
            pkgs = pkgs;
            version = versionFileContents;
          };
        }
        // (
          if pkgs.stdenv.isLinux then
            {
              docker-image = dockerImage { pkgs = pkgs; };
            }
          else
            { }
        )
      );
      # `nix develop` provides a shell containing required tools.
      devShells = forAllSystems (
        { system, pkgs }: {
          default = pkgs.mkShell {
            buildInputs = (devTools pkgs);
          };
        }
      );
    };
}
