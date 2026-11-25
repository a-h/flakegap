# 2. Export the build inputs of uncached derivations

Date: 2026-11-25

## Status

Accepted

## Context

The airgapped machine must be able to rebuild the flake's outputs after changes to the flake's source, and run `nix develop`, without network access. To build a derivation, Nix needs the outputs of the derivation's direct inputs. When those outputs exist, Nix does not look at the inputs of the inputs.

In November 2024, the export realised the direct inputs of each top-level flake output (`nix derivation show <ref>`). The airgapped machine still tried to download packages such as bash and C libraries. A change to the flake's source changes the hash of every derivation built from that source, including intermediate derivations such as a `node_modules` or Python environment derivation. Each of those is rebuilt, and needs its own direct inputs, which were not exported.

A change to the JSON format of `nix derivation show` (see below) was not the cause. The version 4 format was released after November 2024, and the November 2024 parser required exactly one top-level key, so it would have failed with an error on the version 4 format rather than exporting too little.

The export was changed to realise every derivation in the build closure (`nix path-info --recursive --derivation <ref>`). That works, but includes the bootstrap chain and the source of every nixpkgs package. For this repository's `devShells.aarch64-darwin.default`, that is 1,532 derivations, against 14 direct inputs. Large exports took more than 10 hours.

The JSON output of `nix derivation show` has also changed. Nix 2.18 to 2.31 write a map of absolute store paths to derivations with `inputDrvs` fields. Nix 2.35 writes `{"derivations": {...}, "version": 4}` with store path basenames and `inputs.drvs` fields, and omits the output paths of fixed-output derivations. A parser written for one format finds no inputs in the other without an error. `nixcmd/testdata/derivationshow` contains the output of each version, and the tests parse each one.

## Decision

By default (`-build-inputs uncached`), the export:

1. Reads the derivation graph of each flake output with `nix derivation show --recursive`, and parses either format.
2. Starts at the flake output's derivation, and follows input derivations whose outputs no configured binary cache holds. It checks a cache by requesting `<cache>/<hash>.narinfo` with an HTTP HEAD request. It does not follow fixed-output derivations, because their outputs are fixed by their hash.
3. Realises and exports the direct inputs of the flake output's derivation and of each uncached derivation, with their runtime closures.

A public binary cache such as cache.nixos.org cannot hold the outputs of derivations built from the flake's own source, so the airgapped machine must build them, and the export includes their build inputs. When the walk reaches a derivation that a binary cache holds, such as the Go compiler, the export includes its output and does not follow its inputs. The airgapped machine uses the exported Go compiler, so it does not need the C compiler that built the Go compiler, or the compilers that built the C compiler. This excludes the bootstrap chain.

The binary cache check only decides which derivations' build inputs to export. It does not leave out any path that the airgapped machine needs: the export contains the outputs of the flake, their runtime closures, and the selected build inputs with their runtime closures, whether or not a binary cache also holds them. The airgapped machine imports every path from the export, and never contacts a binary cache. What the export leaves out is how cached derivations were built, such as the compilers that built bash, because the airgapped machine uses the exported bash rather than building it.

`-build-inputs all` keeps the previous behavior.

## Alternatives considered

- **Direct inputs of the top-level output only.** This is the November 2024 approach, which misses the inputs of intermediate derivations built from the flake's source.
- **Every derivation in the build closure.** This is the previous default. It is complete, but slow and large.
- **Mark derivations that reference the flake's source in `inputSrcs`.** Sources such as `./node` and `gitignoreSource ./node` are copied to new store paths, so the source path cannot be matched reliably.
- **Modify the source and compare the derivation graphs.** Edits that do not change evaluation, such as appending whitespace, can break evaluation (for example `builtins.readFile ./VERSION` used in a name), and filtered sources may ignore them.
- **Use `nix build --dry-run` against an empty store.** The empty store does not contain the `.drv` files, so Nix cannot plan the build.
- **Use `nix path-info --store <cache>`.** Nix 2.18 and 2.28 print missing paths to stderr and print the rest, while Nix 2.35 fails, so the result depends on the Nix version.

## Consequences

- Exports skip the bootstrap chain and the sources of nixpkgs packages, which reduces their size and duration.
- The airgapped machine can rebuild derivations that depend on the flake's source. It cannot rebuild changes to nixpkgs packages, such as overrides or version bumps, unless the export uses `-build-inputs all`.
- If the flake's own packages are in a configured binary cache, for example a team cache, the export treats them as cached and does not export their inputs. Use `-build-inputs all` in that case.
- Caches that are not served over HTTP or HTTPS, and lookups that fail, are treated as uncached. The export then includes more than it needs, rather than less.
- The export needs network access to the binary caches, which it already needs to realise the build inputs.
- The export requires Nix 2.18 or later. Nix 2.13 does not have `nix derivation show`, and Nix versions more than 12 months old are not supported.
