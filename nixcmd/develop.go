package nixcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"

	"github.com/creack/pty"
)

const developScript = `case $- in *i*) echo "flakegap-interactive-shell-$((6 * 7))" ;; esac
eval "$FLAKEGAP_DEVELOP_COMMAND"
exit $?
`

const developMarker = "flakegap-interactive-shell-42"

// Develop starts an interactive development shell for the flake reference in a pseudo-terminal, as a user running
// `nix develop <ref>` would, then runs command in the shell and exits. An empty command only checks that the
// interactive shell starts. The shell prints a marker that it computes when it is interactive, so the terminal's
// echo of the typed input does not match it.
func Develop(ctx context.Context, stdout, stderr io.Writer, codeDir, ref, command string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	cmd := exec.CommandContext(ctx, nixPath, "develop", ref)
	cmd.Env = append(getEnv(), "TERM=dumb", "FLAKEGAP_DEVELOP_COMMAND="+command)
	cmd.Dir = codeDir
	terminal, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start nix develop in a pseudo-terminal: %w", err)
	}
	defer terminal.Close()

	if _, err = io.WriteString(terminal, developScript); err != nil {
		return fmt.Errorf("failed to write to the development shell: %w", err)
	}
	var output bytes.Buffer
	_, readErr := io.Copy(&output, terminal)
	// Linux returns EIO from the pseudo-terminal once the shell has exited.
	if errors.Is(readErr, syscall.EIO) {
		readErr = nil
	}
	w, closer := ErrorBuffer(stdout, stderr)
	if _, err = w.Write(output.Bytes()); err != nil {
		return fmt.Errorf("failed to buffer development shell output: %w", err)
	}

	err = errors.Join(cmd.Wait(), readErr)
	if err == nil && !strings.Contains(output.String(), developMarker) {
		err = errors.New("nix develop did not start an interactive shell")
	}
	return closer(err)
}
