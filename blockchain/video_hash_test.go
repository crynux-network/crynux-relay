package blockchain

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestSampledVideoFrameIndices(t *testing.T) {
	got1, err := SampledVideoFrameIndices(1)
	if err != nil || !reflect.DeepEqual(got1, []int{0}) {
		t.Fatalf("expected [0], got %v, err=%v", got1, err)
	}
	got5, err := SampledVideoFrameIndices(5)
	if err != nil || !reflect.DeepEqual(got5, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("expected [0 1 2 3 4], got %v, err=%v", got5, err)
	}
	got81, err := SampledVideoFrameIndices(81)
	if err != nil || !reflect.DeepEqual(got81, []int{0, 11, 22, 34, 45, 57, 68, 80}) {
		t.Fatalf("expected [0 11 22 34 45 57 68 80], got %v, err=%v", got81, err)
	}
	got121, err := SampledVideoFrameIndices(121)
	if err != nil || !reflect.DeepEqual(got121, []int{0, 17, 34, 51, 68, 85, 102, 120}) {
		t.Fatalf("expected [0 17 34 51 68 85 102 120], got %v, err=%v", got121, err)
	}
}

func TestCrossLanguageGoldenVectorsMatchPython(t *testing.T) {
	const height, width = 48, 64
	luma := make([]byte, height*width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			luma[y*width+x] = byte((x*3 + y*5) % 256)
		}
	}
	lumaHash, err := HashVideoLumaFrame(luma, width, height)
	if err != nil {
		t.Fatalf("HashVideoLumaFrame error: %v", err)
	}
	if got := hex.EncodeToString(lumaHash[:]); got != "c097384b5a770de9" {
		t.Fatalf("luma golden vector mismatch: got %s, want c097384b5a770de9", got)
	}

	mono := make([]float64, 24000)
	for i := range mono {
		tm := float64(i) / 24000.0
		mono[i] = 0.6 * math.Sin(2.0*math.Pi*(220.0*tm+400.0*tm*tm))
	}
	audioHash := HashVideoAudioPCM(mono)
	if got := hex.EncodeToString(audioHash[:]); got != "703f073a97dd8087" {
		t.Fatalf("audio golden vector mismatch: got %s, want 703f073a97dd8087", got)
	}

	// Interleaved stereo whose arithmetic channel mean equals mono must yield the identical hash.
	rawStereo := make([]byte, len(mono)*2*8)
	for i, m := range mono {
		left := m + 0.125
		right := m - 0.125
		binary.LittleEndian.PutUint64(rawStereo[i*16:i*16+8], math.Float64bits(left))
		binary.LittleEndian.PutUint64(rawStereo[i*16+8:i*16+16], math.Float64bits(right))
	}
	stereoHash, err := HashInterleavedVideoAudioPCMBytes(rawStereo, 2)
	if err != nil {
		t.Fatalf("HashInterleavedVideoAudioPCMBytes error: %v", err)
	}
	if stereoHash != audioHash {
		t.Fatalf("stereo arithmetic mean hash mismatch: got %x, want %x", stereoHash, audioHash)
	}
	if _, err := HashInterleavedVideoAudioPCMBytes(rawStereo[:15], 2); err == nil {
		t.Fatal("expected unaligned PCM byte slice to fail")
	}
	if _, err := HashInterleavedVideoAudioPCMBytes(rawStereo, 0); err == nil {
		t.Fatal("expected zero channels to fail")
	}
}

func TestVerifyUploadedVideoScoreBinding(t *testing.T) {
	visual := make([]byte, 64)
	for i := range visual {
		visual[i] = byte(i + 1)
	}
	audio := []byte{0x70, 0x3f, 0x07, 0x3a, 0x97, 0xdd, 0x80, 0x87}
	committed := append(append([]byte{}, visual...), audio...)
	committedHex := "0x" + hex.EncodeToString(committed)

	if err := VerifyUploadedVideoScore(committedHex, committed, true, 81, 2); err != nil {
		t.Fatalf("expected identical score to pass: %v", err)
	}

	oneBitAudio := append([]byte{}, committed...)
	oneBitAudio[64] ^= 0x01
	if err := VerifyUploadedVideoScore(committedHex, oneBitAudio, true, 81, 2); err != nil {
		t.Fatalf("expected 1-bit audio rounding delta to pass within tolerance 2: %v", err)
	}

	threeBitAudio := append([]byte{}, committed...)
	threeBitAudio[64] ^= 0x07
	if err := VerifyUploadedVideoScore(committedHex, threeBitAudio, true, 81, 2); !errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("expected ErrInvalidUploadedVideo for 3-bit audio mismatch, got %v", err)
	}

	oneBitVisual := append([]byte{}, committed...)
	oneBitVisual[0] ^= 0x01
	if err := VerifyUploadedVideoScore(committedHex, oneBitVisual, true, 81, 2); !errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("expected ErrInvalidUploadedVideo for visual block mismatch, got %v", err)
	}

	if err := VerifyUploadedVideoScore(committedHex, visual, false, 81, 2); !errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("expected ErrInvalidUploadedVideo for stripped audio stream, got %v", err)
	}

	if err := VerifyUploadedVideoScore("not-hex", committed, true, 81, 2); err == nil || errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("expected non-upload internal error for malformed committed score, got %v", err)
	}
}

func TestVideoExecHelperProcess(t *testing.T) {
	switch os.Getenv("GO_TEST_VIDEO_EXEC_HELPER") {
	case "":
		return
	case "exit1":
		fmt.Fprintln(os.Stderr, "moov atom not found")
		os.Exit(1)
	case "exit255":
		fmt.Fprintln(os.Stderr, "Exiting normally, received signal 15.")
		os.Exit(255)
	case "exit123":
		os.Exit(123)
	case "sleep":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
}

func runHelperSubprocess(t *testing.T, mode string) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestVideoExecHelperProcess$")
	cmd.Env = append(os.Environ(), "GO_TEST_VIDEO_EXEC_HELPER="+mode)
	_, err := cmd.Output()
	if err == nil {
		t.Fatalf("expected helper subprocess %q to fail", mode)
	}
	return err
}

func TestClassifyVideoExecError(t *testing.T) {
	activeCtx := context.Background()
	if err := classifyVideoExecError(activeCtx, "ffprobe", exec.ErrNotFound); errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("missing binary must not be classified as invalid uploaded video: %v", err)
	}
	exitErr := &exec.ExitError{Stderr: []byte("moov atom not found")}
	if err := classifyVideoExecError(activeCtx, "ffprobe", exitErr); !errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("ffprobe exit error on active context must be classified as invalid uploaded video, got %v", err)
	}
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := classifyVideoExecError(canceledCtx, "ffmpeg", exitErr); errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("canceled context must not be classified as invalid uploaded video: %v", err)
	}

	realExit1 := runHelperSubprocess(t, "exit1")
	if err := classifyVideoExecError(activeCtx, "ffprobe", realExit1); !errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("normal exit code 1 must be classified as invalid uploaded video, got %v", err)
	}

	realExit255 := runHelperSubprocess(t, "exit255")
	if err := classifyVideoExecError(activeCtx, "ffmpeg video decode", realExit255); errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("ffmpeg caught-signal exit code 255 must be classified as internal error, got %v", err)
	}

	realExit123 := runHelperSubprocess(t, "exit123")
	if err := classifyVideoExecError(activeCtx, "ffmpeg video decode", realExit123); errors.Is(err, ErrInvalidUploadedVideo) {
		t.Fatalf("ffmpeg hard-signal exit code 123 must be classified as internal error, got %v", err)
	}

	if runtime.GOOS != "windows" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestVideoExecHelperProcess$")
		cmd.Env = append(os.Environ(), "GO_TEST_VIDEO_EXEC_HELPER=sleep")
		if err := cmd.Start(); err != nil {
			t.Fatalf("start helper process: %v", err)
		}
		if err := cmd.Process.Kill(); err != nil {
			t.Fatalf("kill helper process: %v", err)
		}
		killErr := cmd.Wait()
		if err := classifyVideoExecError(activeCtx, "ffmpeg video decode", killErr); errors.Is(err, ErrInvalidUploadedVideo) {
			t.Fatalf("SIGKILL process termination must be classified as internal error, got %v", err)
		}
	}
}
