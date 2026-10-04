package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
)

func TestLivez(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok\n" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestEncodeRejectsGET(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/m4a", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rec.Code)
	}
}

func TestEncodeFailingFFmpegReturns422(t *testing.T) {
	old := ffmpegBin
	ffmpegBin = "false" // always exits 1
	t.Cleanup(func() { ffmpegBin = old })

	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/m4a", bytes.NewReader([]byte("junk"))))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422", rec.Code)
	}
}

// toneWAV builds a 16-bit mono PCM WAV holding a 440 Hz sine. Not silence:
// loudnorm turns digital silence into NaN, which the AAC encoder rejects.
func toneWAV(rate, seconds int) []byte {
	data := make([]byte, rate*seconds*2)
	for i := 0; i < rate*seconds; i++ {
		v := int16(8000 * math.Sin(2*math.Pi*440*float64(i)/float64(rate)))
		binary.LittleEndian.PutUint16(data[2*i:], uint16(v))
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(rate * 2), uint16(2), uint16(16)} {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(data)
	return b.Bytes()
}

func TestEncodeProducesM4A(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/m4a?filename=../x/daily.m4a", bytes.NewReader(toneWAV(24000, 2))))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/mp4" {
		t.Fatalf("content-type %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename=daily.m4a` {
		t.Fatalf("content-disposition %q", cd)
	}
	// MP4 files carry an "ftyp" box right after the 4-byte size field.
	if body := rec.Body.Bytes(); len(body) < 8 || string(body[4:8]) != "ftyp" {
		t.Fatalf("output is not an MP4 container")
	}
}

func TestOutputNameDefault(t *testing.T) {
	if got := outputName(httptest.NewRequest(http.MethodPost, "/m4a", nil)); got != "audio.m4a" {
		t.Fatalf("got %q", got)
	}
}
