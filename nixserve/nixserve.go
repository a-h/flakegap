package nixserve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/a-h/flakegap/nixserve/db"
)

func New(log *slog.Logger) (h *Handler, closer func() error, err error) {
	db, closer, err := db.New("/nix")
	h = &Handler{
		Log: log,
		db:  db,
	}
	return h, closer, err
}

type Handler struct {
	Log *slog.Logger
	db  *db.DB
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	msg := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	status := http.StatusOK
	var err error
	defer func() {
		if err != nil {
			h.Log.Error(msg, slog.Any("status", status), slog.Int("ms", int(time.Since(now).Milliseconds())), slog.Any("error", err))
			return
		}
		h.Log.Info(msg, slog.Any("status", status), slog.Int("ms", int(time.Since(now).Milliseconds())))
	}()
	if r.URL.Path == "/nix-cache-info" {
		status, err = h.getNixCacheInfo(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, ".narinfo") {
		status, err = h.getNarInfo(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, ".nar") {
		status, err = h.getNar(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/log/") {
		status, err = h.getLog(w, r)
		return
	}
	status = http.StatusNotFound
	http.NotFound(w, r)
}

func (h *Handler) getNixCacheInfo(w http.ResponseWriter, r *http.Request) (status int, err error) {
	if !strings.HasSuffix(r.URL.Path, "nix-cache-info") {
		http.Error(w, fmt.Sprintf("expected nix-cache-info file, got %s\n", r.URL.Path), http.StatusBadRequest)
		return http.StatusBadRequest, nil
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "StoreDir: %s\nWantMassQuery: 1\nPriority: 30\n", h.db.StorePath)
	return http.StatusOK, nil
}

func (h *Handler) getNarInfo(w http.ResponseWriter, r *http.Request) (status int, err error) {
	if !strings.HasSuffix(r.URL.Path, ".narinfo") {
		http.Error(w, fmt.Sprintf("expected .narinfo file, got %s\n", r.URL.Path), http.StatusBadRequest)
		return http.StatusBadRequest, nil
	}
	fileName := path.Base(r.URL.Path)
	hashPart := strings.TrimSuffix(fileName, ".narinfo")
	storePath, ok, err := h.db.QueryPathFromHashPart(r.Context(), hashPart)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query path: %v\n", err), http.StatusInternalServerError)
		return http.StatusInternalServerError, fmt.Errorf("failed to query path: %w", err)
	}
	if !ok {
		http.Error(w, fmt.Sprintf("path not found for: %s\n", hashPart), http.StatusNotFound)
		return http.StatusNotFound, nil
	}
	pathInfo, ok, err := h.db.QueryPathInfo(r.Context(), storePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query path info: %v\n", err), http.StatusInternalServerError)
		return http.StatusInternalServerError, fmt.Errorf("failed to query path info: %w", err)
	}
	if !ok {
		http.Error(w, fmt.Sprintf("path info not found for %s\n", storePath), http.StatusNotFound)
		return http.StatusNotFound, nil
	}
	narHashParts := strings.SplitN(pathInfo.Hash, ":", 2)
	if len(narHashParts) != 2 {
		http.Error(w, fmt.Sprintf("invalid hash: %s\n", pathInfo.Hash), http.StatusInternalServerError)
		return http.StatusInternalServerError, fmt.Errorf("invalid hash: %s", pathInfo.Hash)
	}
	narHash := narHashParts[1]

	w.Header().Set("Content-Type", "text/x-nix-narinfo")

	// Create the output.
	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, "StorePath: %s\n", storePath)
	fmt.Fprintf(buf, "URL: nar/%s-%s.nar\n", hashPart, narHash)
	fmt.Fprintf(buf, "Compression: none\n")
	fmt.Fprintf(buf, "NarHash: %s\n", pathInfo.Hash)
	fmt.Fprintf(buf, "NarSize: %d\n", pathInfo.NarSize)
	if len(pathInfo.Refs) > 0 {
		fmt.Fprintf(buf, "References: %s\n", strings.Join(pathInfo.Refs, " "))
	}
	if pathInfo.Deriver != "" {
		fmt.Fprintf(buf, "Deriver: %s\n", pathInfo.Deriver)
	}

	// Send the output.
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.Write(buf.Bytes())

	return http.StatusOK, nil
}

func (h *Handler) getNar(w http.ResponseWriter, r *http.Request) (status int, err error) {
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
		hashPart = split[0]
	}

	storePath, ok, err := h.db.QueryPathFromHashPart(r.Context(), hashPart)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query path: %v\n", err), http.StatusInternalServerError)
		return http.StatusInternalServerError, fmt.Errorf("failed to query path: %w", err)
	}
	if !ok {
		http.Error(w, fmt.Sprintf("path not found for %s\n", hashPart), http.StatusNotFound)
		return http.StatusNotFound, nil
	}
	pathInfo, ok, err := h.db.QueryPathInfo(r.Context(), storePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to query path info: %v\n", err), http.StatusInternalServerError)
		return http.StatusInternalServerError, fmt.Errorf("failed to query path info: %w", err)
	}
	if !ok {
		http.Error(w, fmt.Sprintf("path info not found for %s\n", storePath), http.StatusNotFound)
		return http.StatusNotFound, nil
	}
	if expectedNarHash != "" && expectedNarHash != pathInfo.Hash {
		http.Error(w, "Incorrect NAR hash. Maybe the path has been recreated.", http.StatusNotFound)
		return http.StatusNotFound, fmt.Errorf("incorrect hash: expected %s, actual %s", expectedNarHash, pathInfo.Hash)
	}

	// The Perl implementation sets the Content-Type to text/plain,
	// but it should be application/octet-stream.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", pathInfo.NarSize))

	stderr := bytes.NewBuffer(nil)
	if err = dumpPath(r.Context(), w, stderr, storePath); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to dump path %q with stderr %q: %w", storePath, stderr.String(), err)
	}
	return http.StatusOK, nil
}

func dumpPath(ctx context.Context, stdout, stderr io.Writer, ref string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %w", err)
	}
	cmdArgs := []string{"store", "dump-path", ref}
	cmd := exec.CommandContext(ctx, nixPath, cmdArgs...)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}

func (h *Handler) getLog(w http.ResponseWriter, r *http.Request) (status int, err error) {
	storePath := strings.TrimPrefix(r.URL.Path, "/log")
	stderr := bytes.NewBuffer(nil)
	if err := nixLog(r.Context(), w, stderr, storePath); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to get log for store path %q with stderr %q: %w", storePath, stderr.String(), err)
	}
	return http.StatusOK, nil
}

func nixLog(ctx context.Context, stdout, stderr io.Writer, storePath string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %w", err)
	}
	cmd := exec.CommandContext(ctx, nixPath, "log", storePath)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}
