package nixserve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path"
	"strings"

	"github.com/a-h/flakegap/nixserve/db"
)

func New() (h *Handler, closer func() error, err error) {
	db, closer, err := db.New("file:/nix/var/nix/db/db.sqlite")
	h = &Handler{
		db:        db,
		storePath: "/nix/store",
	}
	return h, closer, err
}

type Handler struct {
	db        *db.DB
	storePath string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/nix-cache-info" {
		h.getNixCacheInfo(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, ".narinfo") {
		h.getNarInfo(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, ".nar") {
		h.getNar(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/log/") {
		h.getLog(w, r)
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) getNixCacheInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "StoreDir: %s\nWantMassQuery: 1\nPriority: 30\n", h.storePath)
}

func (h *Handler) getNarInfo(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, ".narinfo") {
		http.Error(w, fmt.Sprintf("expected .narinfo file, got %s\n", r.URL.Path), http.StatusBadRequest)
		return
	}
	fileName := path.Base(r.URL.Path)
	hashPart := strings.TrimSuffix(fileName, ".narinfo")
	storePath, ok, err := h.db.QueryPathFromHashPart(r.Context(), hashPart)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query path: %v\n", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "narinfo not found for %s\n", hashPart)
		return
	}
	pathInfo, ok, err := h.db.QueryPathInfo(r.Context(), storePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query path info: %v\n", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, fmt.Sprintf("path info not found for %s\n", storePath), http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(pathInfo.Hash, "sha256:") {
		http.Error(w, fmt.Sprintf("invalid hash: %s\n", pathInfo.Hash), http.StatusInternalServerError)
		return
	}
	narHash := strings.TrimPrefix(pathInfo.Hash, "sha256:")
	if len(narHash) != 52 {
		http.Error(w, fmt.Sprintf("invalid hash length: %s\n", pathInfo.Hash), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/x-nix-narinfo")

	// Create the output.
	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, "StorePath: %s\n", storePath)
	fmt.Fprintf(buf, "URL: nar/%s-%s.nar\n", hashPart, narHash)
	fmt.Fprintf(buf, "Compression: none\n")
	fmt.Fprintf(buf, "NarHash: %s\n", pathInfo.Hash)
	fmt.Fprintf(buf, "NarSize: %d\n", pathInfo.NarSize)
	fmt.Fprintf(buf, "References: %s\n", strings.Join(pathInfo.Refs, " "))
	//TODO: Implement the derivers field.

	// Send the output.
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.Write(buf.Bytes())
}

func (h *Handler) getNar(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, ".nar") {
		http.Error(w, fmt.Sprintf("expected .nar file, got %s\n", r.URL.Path), http.StatusBadRequest)
		return
	}

	// Get the hash part.
	fileName := path.Base(r.URL.Path)
	hashPart := strings.TrimSuffix(fileName, ".nar")

	// Get the expected Nar hash if the filename has one.
	var expectedNarHash string
	if split := strings.SplitN(hashPart, "-", 2); len(split) == 2 {
		expectedNarHash = "sha256:" + split[1]
	}

	storePath, ok, err := h.db.QueryPathFromHashPart(r.Context(), hashPart)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query path: %v\n", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "narinfo not found for %s\n", hashPart)
		return
	}
	pathInfo, ok, err := h.db.QueryPathInfo(r.Context(), storePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query path info: %v\n", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, fmt.Sprintf("path info not found for %s\n", storePath), http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(pathInfo.Hash, "sha256:") {
		http.Error(w, fmt.Sprintf("invalid hash: %s\n", pathInfo.Hash), http.StatusInternalServerError)
		return
	}
	narHash := strings.TrimPrefix(pathInfo.Hash, "sha256:")
	if len(narHash) != 52 {
		http.Error(w, fmt.Sprintf("invalid hash length: %s\n", pathInfo.Hash), http.StatusInternalServerError)
		return
	}
	if expectedNarHash != "" && expectedNarHash != pathInfo.Hash {
		http.Error(w, "Incorrect NAR hash. Maybe the path has been recreated.", http.StatusNotFound)
		return
	}

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	if err = dumpPath(r.Context(), stdout, stderr, storePath); err != nil {
		http.Error(w, fmt.Sprintf("failed to dump path with stdout %q and stderr %q", string(stdout.Bytes()), string(stderr.Bytes())), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stdout.Len()))
	w.Write(stdout.Bytes())
}

func dumpPath(ctx context.Context, stdout, stderr io.Writer, ref string, args ...string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	cmdArgs := append([]string{"run", ref}, args...)
	cmd := exec.CommandContext(ctx, nixPath, cmdArgs...)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}

func (h *Handler) getLog(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, ".nar") {
		http.Error(w, fmt.Sprintf("expected .nar file, got %s\n", r.URL.Path), http.StatusBadRequest)
		return
	}
}

func nixLog(ctx context.Context, stdout, stderr io.Writer, storePath string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	cmd := exec.CommandContext(ctx, nixPath, "log", storePath)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}
