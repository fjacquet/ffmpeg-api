// Command ffmpeg-api is an internal HTTP sidecar that encodes a WAV upload into an
// iOS-friendly AAC/M4A file. It has no authentication: never expose it outside a
// private Docker network.
package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var version = "dev"

const (
	listenAddr    = ":8080"
	maxBody       = 60 << 20 // 60 MiB, ~20 min of 24 kHz mono 16-bit WAV
	encodeTimeout = 2 * time.Minute
)

// ffmpegBin is a variable so tests can point it at a stub.
var ffmpegBin = "ffmpeg"

func encodeM4A(w http.ResponseWriter, r *http.Request) {
	dir, err := os.MkdirTemp("", "enc-")
	if err != nil {
		http.Error(w, "cannot create temp dir", http.StatusInternalServerError)
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()
	in, out := filepath.Join(dir, "in.wav"), filepath.Join(dir, "out.m4a")

	f, err := os.Create(in)
	if err != nil {
		http.Error(w, "cannot create input file", http.StatusInternalServerError)
		return
	}
	_, err = io.Copy(f, http.MaxBytesReader(w, r.Body, maxBody))
	if cerr := f.Close(); err == nil && cerr != nil {
		http.Error(w, "cannot write input file", http.StatusInternalServerError)
		return
	}
	if err != nil {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), encodeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpegBin, "-nostdin", "-loglevel", "error", "-i", in,
		"-af", "loudnorm=I=-16:TP=-1.5:LRA=11", "-ar", "24000", "-ac", "1",
		"-c:a", "aac", "-b:a", "40k", "-movflags", "+faststart", out)
	if msg, err := cmd.CombinedOutput(); err != nil {
		log.Printf("ffmpeg failed: %v: %s", err, msg)
		http.Error(w, "ffmpeg could not encode the input", http.StatusUnprocessableEntity)
		return
	}

	res, err := os.Open(out)
	if err != nil {
		http.Error(w, "cannot read encoded file", http.StatusInternalServerError)
		return
	}
	defer func() { _ = res.Close() }()
	w.Header().Set("Content-Type", "audio/mp4")
	if _, err := io.Copy(w, res); err != nil {
		log.Printf("write response: %v", err)
	}
}

func livez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /m4a", encodeM4A)
	mux.HandleFunc("GET /livez", livez)
	return mux
}

func main() {
	log.Printf("ffmpeg-api %s listening on %s", version, listenAddr)
	srv := &http.Server{Addr: listenAddr, Handler: newMux(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
