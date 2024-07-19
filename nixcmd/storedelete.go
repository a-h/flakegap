package nixcmd

import (
	"fmt"
	"io"
	"os/exec"
)

func StoreDelete(stdout, stderr io.Writer, paths []string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	cmd := exec.Command(nixPath, append([]string{"store", "delete"}, paths...)...)
	cmd.Dir = "/code"
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}
