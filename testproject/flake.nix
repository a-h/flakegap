{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    gitignore = {
      url = "github:hercules-ci/gitignore.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { nixpkgs, gitignore, ... }:
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

      app = { name, pkgs, system, ... }: pkgs.buildNpmPackage {
        pname = name;
        version = "0.0.1";

        src = gitignore.lib.gitignoreSource ./node;

        npmDeps = pkgs.importNpmLock {
          npmRoot = gitignore.lib.gitignoreSource ./node;
        };

        npmConfigHook = pkgs.importNpmLock.npmConfigHook;

        buildPhase = ''
          runHook preBuild

          npm run build
          # Copy the build to the output directory.
          mkdir -p $out
          cp -r ./build $out
          # Copy the package.json to the output directory.
          cp -r ./package.json $out
          # Copy node_modules to the output directory.
          cp -r ./node_modules $out

          echo "Creating entrypoint script..."
          mkdir -p $out/bin
          cat << EOF > $out/bin/${name}
          #!${pkgs.bash}/bin/bash
          ${pkgs.node}/bin/node $out
          EOF
          chmod +x $out/bin/${name}

          runHook postBuild
        '';
      };

      # Build Docker containers.
      dockerUser = pkgs: pkgs.runCommand "user" { } ''
        mkdir -p $out/etc
        echo "user:x:1000:1000:user:/home/user:/bin/false" > $out/etc/passwd
        echo "user:x:1000:" > $out/etc/group
        echo "user:!:1::::::" > $out/etc/shadow
      '';

      appDockerImage = { name, pkgs, system }: pkgs.dockerTools.buildImage {
        name = name;
        tag = "latest";

        copyToRoot = [
          # Remove coreutils and bash for a smaller container.
          pkgs.coreutils
          pkgs.bash
          pkgs.curl
          (dockerUser pkgs)
          (app { inherit name pkgs system; })
        ];
        config = {
          Entrypoint = [ "/bin/${name}" ];
          User = "user:user";
        };
      };

      # Development tools used.
      devTools = { system, pkgs }: [
        pkgs.gh
        pkgs.git
        # Container tools.
        pkgs.crane
        pkgs.docker
      ];

      name = "testproject";
    in
    {
      packages = forAllSystems ({ system, pkgs }: {
        default = app {
          name = name;
          pkgs = pkgs;
          system = system;
        };
        app-docker-image = appDockerImage { name = name; pkgs = pkgs; system = system; };
      });

      # `nix develop` provides a shell containing required tools.
      devShells = forAllSystems ({ system, pkgs }: {
        default = pkgs.mkShell {
          buildInputs = (devTools { system = system; pkgs = pkgs; });
        };
      });
    };
}

