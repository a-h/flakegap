package nixcmd

import "os"

func getEnv() (env []string) {
	// HOME is required for git to find the user's global gitconfig.
	if os.Getenv("HOME") == "" {
		env = append(env, "HOME=/root")
	} else {
		env = append(env, "HOME="+os.Getenv("HOME"))
	}
	// NIXPKGS_ALLOW_UNFREE is required for nix to build unfree packages such as Terraform.
	env = append(env, "NIXPKGS_ALLOW_UNFREE=1")
	return env
}
