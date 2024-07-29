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
	h.Log.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
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
	if !strings.HasSuffix(r.URL.Path, "nix-cache-info") {
		http.Error(w, fmt.Sprintf("expected nix-cache-info file, got %s\n", r.URL.Path), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "StoreDir: %s\nWantMassQuery: 1\nPriority: 30\n", h.db.StorePath)
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
		h.Log.Error("failed to query path", slog.Any("error", err))
		http.Error(w, fmt.Sprintf("failed to query path: %v\n", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		h.Log.Info("narinfo not found", slog.Any("hashPart", hashPart))
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "narinfo not found for %s\n", hashPart)
		return
	}
	pathInfo, ok, err := h.db.QueryPathInfo(r.Context(), storePath)
	if err != nil {
		h.Log.Error("failed to query path info", slog.Any("error", err))
		http.Error(w, fmt.Sprintf("failed to query path info: %v\n", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		h.Log.Warn("path info not found", slog.Any("storePath", storePath))
		http.Error(w, fmt.Sprintf("path info not found for %s\n", storePath), http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(pathInfo.Hash, "sha256:") {
		h.Log.Warn("invalid hash", slog.Any("hash", pathInfo.Hash))
		http.Error(w, fmt.Sprintf("invalid hash: %s\n", pathInfo.Hash), http.StatusInternalServerError)
		return
	}
	narHash := strings.TrimPrefix(pathInfo.Hash, "sha256:")
	if len(narHash) != 64 {
		h.Log.Warn("invalid hash length", slog.Any("hash", pathInfo.Hash))
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
	if pathInfo.Deriver != "" {
		fmt.Fprintf(buf, "Deriver: %s\n", pathInfo.Deriver)
	}

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
		hashPart = split[0]
	}

	storePath, ok, err := h.db.QueryPathFromHashPart(r.Context(), hashPart)
	if err != nil {
		h.Log.Error("failed to query path", slog.Any("error", err))
		http.Error(w, fmt.Sprintf("failed to query path: %v\n", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		h.Log.Info("narinfo not found", slog.Any("hashPart", hashPart))
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "narinfo not found for %s\n", hashPart)
		return
	}
	pathInfo, ok, err := h.db.QueryPathInfo(r.Context(), storePath)
	if err != nil {
		h.Log.Error("failed to query path info", slog.Any("error", err))
		http.Error(w, fmt.Sprintf("failed to query path info: %v\n", err), http.StatusInternalServerError)
		return
	}
	if !ok {
		h.Log.Warn("path info not found", slog.Any("storePath", storePath))
		http.Error(w, fmt.Sprintf("path info not found for %s\n", storePath), http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(pathInfo.Hash, "sha256:") {
		h.Log.Warn("invalid hash", slog.Any("hash", pathInfo.Hash))
		http.Error(w, fmt.Sprintf("invalid hash: %s\n", pathInfo.Hash), http.StatusInternalServerError)
		return
	}
	narHash := strings.TrimPrefix(pathInfo.Hash, "sha256:")
	if len(narHash) != 64 {
		h.Log.Warn("invalid hash length", slog.Any("hash", pathInfo.Hash))
		http.Error(w, fmt.Sprintf("invalid hash length: %s\n", pathInfo.Hash), http.StatusInternalServerError)
		return
	}
	if expectedNarHash != "" && expectedNarHash != pathInfo.Hash {
		h.Log.Warn("incorrect hash", slog.Any("expected", expectedNarHash), slog.Any("actual", pathInfo.Hash))
		http.Error(w, "Incorrect NAR hash. Maybe the path has been recreated.", http.StatusNotFound)
		return
	}

	// The Perl implementation sets the Content-Type to text/plain,
	// but it should be application/octet-stream.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", pathInfo.NarSize))

	stderr := bytes.NewBuffer(nil)
	if err = dumpPath(r.Context(), w, stderr, storePath); err != nil {
		h.Log.Error("failed to dump path", slog.String("storePath", storePath), slog.Any("error", err), slog.String("stderr", stderr.String()))
		return
	}
}

func dumpPath(ctx context.Context, stdout, stderr io.Writer, ref string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}
	cmdArgs := []string{"store", "dump-path", ref}
	cmd := exec.CommandContext(ctx, nixPath, cmdArgs...)
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}

func (h *Handler) getLog(w http.ResponseWriter, r *http.Request) {
	storePath := strings.TrimPrefix(r.URL.Path, "/log")
	stderr := bytes.NewBuffer(nil)
	if err := nixLog(r.Context(), w, stderr, storePath); err != nil {
		h.Log.Error("failed to get log", slog.String("storePath", r.URL.Path), slog.Any("error", err), slog.String("stderr", stderr.String()))
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
