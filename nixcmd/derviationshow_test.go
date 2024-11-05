package nixcmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

var drvJSON = `{
  "/nix/store/6m2j1f6yk9m0gmv6r4pp4ibmjq9j3vnk-flakegap.drv": {
    "args": [
      "-e",
      "/nix/store/v6x3cs394jgqfbi0a42pam708flxaphh-default-builder.sh"
    ],
    "builder": "/nix/store/vmk8yr9fny5lfgqhqdn7igp4fqlyl793-bash-5.2p32/bin/bash",
    "env": {
      "CGO_ENABLED": "0",
      "GO111MODULE": "on",
      "GOARCH": "arm64",
      "GOFLAGS": "-mod=vendor -trimpath",
      "GOOS": "darwin",
      "GO_NO_VENDOR_CHECKS": "1",
      "__darwinAllowLocalNetworking": "",
      "__impureHostDeps": "/bin/sh /usr/lib/libSystem.B.dylib /usr/lib/system/libunc.dylib /dev/zero /dev/random /dev/urandom /bin/sh",
      "__propagatedImpureHostDeps": "",
      "__propagatedSandboxProfile": "",
      "__sandboxProfile": "",
      "__structuredAttrs": "",
      "buildInputs": "",
      "buildPhase": "runHook preBuild\n\nexclude='\\(/_\\|examples\\|Godeps\\|testdata'\nif [[ -n \"$excludedPackages\" ]]; then\n  IFS=' ' read -r -a excludedArr <<<$excludedPackages\n  printf -v excludedAlternates '%s\\\\|' \"${excludedArr[@]}\"\n  excludedAlternates=${excludedAlternates%\\\\|} # drop final \\| added by printf\n  exclude+='\\|'\"$excludedAlternates\"\nfi\nexclude+='\\)'\n\nbuildGoDir() {\n  local cmd=\"$1\" dir=\"$2\"\n\n  . $TMPDIR/buildFlagsArray\n\n  declare -a flags\n  flags+=($buildFlags \"${buildFlagsArray[@]}\")\n  flags+=(${tags:+-tags=})\n  flags+=(${ldflags:+-ldflags=\"$ldflags\"})\n  flags+=(\"-v\" \"-p\" \"$NIX_BUILD_CORES\")\n\n  if [ \"$cmd\" = \"test\" ]; then\n    flags+=(-vet=off)\n    flags+=($checkFlags)\n  fi\n\n  local OUT\n  if ! OUT=\"$(go $cmd \"${flags[@]}\" $dir 2>&1)\"; then\n    if echo \"$OUT\" | grep -qE 'imports .*?: no Go files in'; then\n      echo \"$OUT\" >&2\n      return 1\n    fi\n    if ! echo \"$OUT\" | grep -qE '(no( buildable| non-test)?|build constraints exclude all) Go (source )?files'; then\n      echo \"$OUT\" >&2\n      return 1\n    fi\n  fi\n  if [ -n \"$OUT\" ]; then\n    echo \"$OUT\" >&2\n  fi\n  return 0\n}\n\ngetGoDirs() {\n  local type;\n  type=\"$1\"\n  if [ -n \"$subPackages\" ]; then\n    echo \"$subPackages\" | sed \"s,\\(^\\| \\),\\1./,g\"\n  else\n    find . -type f -name \\*$type.go -exec dirname {} \\; | grep -v \"/vendor/\" | sort --unique | grep -v \"$exclude\"\n  fi\n}\n\nif (( \"${NIX_DEBUG:-0}\" >= 1 )); then\n  buildFlagsArray+=(-x)\nfi\n\nif [ ${#buildFlagsArray[@]} -ne 0 ]; then\n  declare -p buildFlagsArray > $TMPDIR/buildFlagsArray\nelse\n  touch $TMPDIR/buildFlagsArray\nfi\nif [ -z \"$enableParallelBuilding\" ]; then\n    export NIX_BUILD_CORES=1\nfi\nfor pkg in $(getGoDirs \"\"); do\n  echo \"Building subPackage $pkg\"\n  buildGoDir install \"$pkg\"\ndone\nrunHook postBuild\n",
      "builder": "/nix/store/vmk8yr9fny5lfgqhqdn7igp4fqlyl793-bash-5.2p32/bin/bash",
      "checkPhase": "runHook preCheck\n\n# We do not set trimpath for tests, in case they reference test assets\nexport GOFLAGS=${GOFLAGS//-trimpath/}\n\nfor pkg in $(getGoDirs test); do\n  buildGoDir test \"$pkg\"\ndone\n\nrunHook postCheck\n",
      "cmakeFlags": "",
      "configureFlags": "",
      "configurePhase": "runHook preConfigure\n\nexport GOCACHE=$TMPDIR/go-cache\nexport GOPATH=\"$TMPDIR/go\"\nexport GOSUMDB=off\nexport GOPROXY=off\ncd \"$modRoot\"\n\nif [ -n \"/nix/store/zlaf653bw3m1z25dxy7fw6wbipgxsncf-vendor-env\" ]; then\n  rm -rf vendor\n  rsync -a -K --ignore-errors /nix/store/zlaf653bw3m1z25dxy7fw6wbipgxsncf-vendor-env/ vendor\nfi\n\n\nrunHook postConfigure\n",
      "depsBuildBuild": "",
      "depsBuildBuildPropagated": "",
      "depsBuildTarget": "",
      "depsBuildTargetPropagated": "",
      "depsHostHost": "",
      "depsHostHostPropagated": "",
      "depsTargetTarget": "",
      "depsTargetTargetPropagated": "",
      "disallowedReferences": "/nix/store/drlw9cnyz3plmngvjcscyxqlyjd71anp-go-1.22.6",
      "doCheck": "1",
      "doInstallCheck": "",
      "flags": "-trimpath",
      "go": "/nix/store/drlw9cnyz3plmngvjcscyxqlyjd71anp-go-1.22.6",
      "installPhase": "runHook preInstall\n\nmkdir -p $out\ndir=\"$GOPATH/bin\"\n[ -e \"$dir\" ] && cp -r $dir $out\n\nrunHook postInstall\n",
      "ldflags": "-s -w -extldflags -static -X main.version=20241105202228",
      "mesonFlags": "",
      "name": "flakegap",
      "nativeBuildInputs": "/nix/store/f2w6bm4bpgdl3ih1ch6m3dacr3l8fawv-rsync-3.3.0 /nix/store/drlw9cnyz3plmngvjcscyxqlyjd71anp-go-1.22.6",
      "out": "/nix/store/kxdcx7rj32xrky6g60k8lpd69mlwznxl-flakegap",
      "outputs": "out",
      "patches": "",
      "propagatedBuildInputs": "",
      "propagatedNativeBuildInputs": "",
      "pwd": "/nix/store/638nnx35i53zrkb36nyp11ag7r1qfk8d-hy1lxxc23z6j9kp6h57672plbixykxwn-source",
      "src": "/nix/store/hy1lxxc23z6j9kp6h57672plbixykxwn-source",
      "stdenv": "/nix/store/h3sjynwq0s33d60f7r69bidiyb9ba0wl-stdenv-darwin",
      "strictDeps": "1",
      "subPackages": "cmd/flakegap",
      "system": "aarch64-darwin"
    },
    "inputDrvs": {
      "/nix/store/63ikirp1jv5c691gvlj82zycxmczvxvg-rsync-3.3.0.drv": {
        "dynamicOutputs": {},
        "outputs": [
          "out"
        ]
      },
      "/nix/store/c73kpfz1iyhaa9w5mpg0c98ap5siwb0p-bash-5.2p32.drv": {
        "dynamicOutputs": {},
        "outputs": [
          "out"
        ]
      },
      "/nix/store/n4smqd5lj7gfm7xbjj2h3jjvgiwwcjzm-vendor-env.drv": {
        "dynamicOutputs": {},
        "outputs": [
          "out"
        ]
      },
      "/nix/store/pv4j66d9f5k9x9rk0wdqh4pg4j6lgcjj-stdenv-darwin.drv": {
        "dynamicOutputs": {},
        "outputs": [
          "out"
        ]
      },
      "/nix/store/vnizn5lspyzscn4l7m34is5b7fpdfb2b-go-1.22.6.drv": {
        "dynamicOutputs": {},
        "outputs": [
          "out"
        ]
      }
    },
    "inputSrcs": [
      "/nix/store/638nnx35i53zrkb36nyp11ag7r1qfk8d-hy1lxxc23z6j9kp6h57672plbixykxwn-source",
      "/nix/store/hy1lxxc23z6j9kp6h57672plbixykxwn-source",
      "/nix/store/v6x3cs394jgqfbi0a42pam708flxaphh-default-builder.sh"
    ],
    "name": "flakegap",
    "outputs": {
      "out": {
        "path": "/nix/store/kxdcx7rj32xrky6g60k8lpd69mlwznxl-flakegap"
      }
    },
    "system": "aarch64-darwin"
  }
}
`

func TestGetInputDrvs(t *testing.T) {
	drvs, err := getInputDrvs([]byte(drvJSON))
	if err != nil {
		t.Fatalf("failed to unmarshal derivation: %v", err)
	}
	expected := []string{
		"/nix/store/63ikirp1jv5c691gvlj82zycxmczvxvg-rsync-3.3.0.drv",
		"/nix/store/c73kpfz1iyhaa9w5mpg0c98ap5siwb0p-bash-5.2p32.drv",
		"/nix/store/n4smqd5lj7gfm7xbjj2h3jjvgiwwcjzm-vendor-env.drv",
		"/nix/store/pv4j66d9f5k9x9rk0wdqh4pg4j6lgcjj-stdenv-darwin.drv",
		"/nix/store/vnizn5lspyzscn4l7m34is5b7fpdfb2b-go-1.22.6.drv",
	}
	if diff := cmp.Diff(expected, drvs); diff != "" {
		t.Error(diff)
	}
}
