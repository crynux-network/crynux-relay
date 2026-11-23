package blockchain

import (
	"bytes"
	"context"
	"crynux_relay/models"
	"crynux_relay/utils"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

var ErrInvalidUploadedVideo = errors.New("invalid uploaded video")

const (
	MaxVideoScoreFrames    = 8
	videoDCTGridSize       = 32
	videoDCTLowFreqSize    = 8
	videoAudioTimeSegments = 32
	videoAudioFreqBands    = 32
	videoAudioWindowSize   = 64
	videoAudioMaxWindows   = 64
	videoAudioLogEpsilon   = 1e-10
)

var (
	videoDCT2DLowBasis [videoDCTLowFreqSize][videoDCTGridSize]float64
	videoAudioHann     [videoAudioWindowSize]float64
	videoAudioDCTBasis [videoAudioFreqBands][videoAudioWindowSize]float64
)

func init() {
	for u := 0; u < videoDCTLowFreqSize; u++ {
		scale := math.Sqrt(2.0 / float64(videoDCTGridSize))
		if u == 0 {
			scale = math.Sqrt(1.0 / float64(videoDCTGridSize))
		}
		for k := 0; k < videoDCTGridSize; k++ {
			videoDCT2DLowBasis[u][k] = scale * math.Cos(math.Pi*float64(2*k+1)*float64(u)/float64(2*videoDCTGridSize))
		}
	}
	for n := 0; n < videoAudioWindowSize; n++ {
		videoAudioHann[n] = 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(n)/float64(videoAudioWindowSize)))
	}
	for band := 0; band < videoAudioFreqBands; band++ {
		for n := 0; n < videoAudioWindowSize; n++ {
			videoAudioDCTBasis[band][n] = math.Cos(math.Pi * float64(2*n+1) * float64(band) / float64(2*videoAudioWindowSize))
		}
	}
}

// ExpectedVideoTaskSpec parses expected num_frames and optional explicit width/height from VideoTaskArgs JSON.
func ExpectedVideoTaskSpec(taskArgs string) (numFrames, width, height int, err error) {
	var parsed struct {
		TaskConfig *struct {
			Width     *int `json:"width"`
			Height    *int `json:"height"`
			NumFrames *int `json:"num_frames"`
		} `json:"task_config"`
	}
	if err := json.Unmarshal([]byte(taskArgs), &parsed); err != nil {
		return 0, 0, 0, fmt.Errorf("parse video task args: %w", err)
	}
	numFrames = models.DefaultVideoNumFrames
	if parsed.TaskConfig != nil {
		if parsed.TaskConfig.NumFrames != nil {
			numFrames = *parsed.TaskConfig.NumFrames
		}
		if parsed.TaskConfig.Width != nil {
			width = *parsed.TaskConfig.Width
		}
		if parsed.TaskConfig.Height != nil {
			height = *parsed.TaskConfig.Height
		}
	}
	if numFrames <= 0 {
		return 0, 0, 0, errors.New("video num_frames must be positive")
	}
	return numFrames, width, height, nil
}

// SampledVideoFrameIndices returns K = min(8, numFrames) uniformly spaced 0-based frame indices.
func SampledVideoFrameIndices(numFrames int) ([]int, error) {
	if numFrames <= 0 {
		return nil, fmt.Errorf("numFrames must be positive, got %d", numFrames)
	}
	count := MaxVideoScoreFrames
	if numFrames < count {
		count = numFrames
	}
	if count == 1 {
		return []int{0}, nil
	}
	indices := make([]int, count)
	for i := 0; i < count; i++ {
		indices[i] = (i * (numFrames - 1)) / (count - 1)
	}
	return indices, nil
}

// DownsampleLuma32x32 computes the exact partition box-average of a 2D 8-bit luma plane.
func DownsampleLuma32x32(luma []byte, width, height int) ([videoDCTGridSize][videoDCTGridSize]float64, error) {
	var grid [videoDCTGridSize][videoDCTGridSize]float64
	if width <= 0 || height <= 0 {
		return grid, errors.New("invalid luma dimensions")
	}
	if len(luma) != width*height {
		return grid, fmt.Errorf("luma byte length %d does not match %dx%d", len(luma), width, height)
	}
	for r := 0; r < videoDCTGridSize; r++ {
		y0 := (r * height) / videoDCTGridSize
		y1 := ((r + 1) * height) / videoDCTGridSize
		if y1 <= y0 {
			y1 = y0 + 1
			if y1 > height {
				y1 = height
			}
		}
		for c := 0; c < videoDCTGridSize; c++ {
			x0 := (c * width) / videoDCTGridSize
			x1 := ((c + 1) * width) / videoDCTGridSize
			if x1 <= x0 {
				x1 = x0 + 1
				if x1 > width {
					x1 = width
				}
			}
			var sum int64
			for y := y0; y < y1; y++ {
				rowOffset := y * width
				for x := x0; x < x1; x++ {
					sum += int64(luma[rowOffset+x])
				}
			}
			count := (y1 - y0) * (x1 - x0)
			grid[r][c] = float64(sum) / float64(count)
		}
	}
	return grid, nil
}

func dct8x8MedianHash(grid [videoDCTGridSize][videoDCTGridSize]float64) [8]byte {
	var rowDCT [videoDCTGridSize][videoDCTLowFreqSize]float64
	for r := 0; r < videoDCTGridSize; r++ {
		for v := 0; v < videoDCTLowFreqSize; v++ {
			acc := 0.0
			for c := 0; c < videoDCTGridSize; c++ {
				acc += grid[r][c] * videoDCT2DLowBasis[v][c]
			}
			rowDCT[r][v] = acc
		}
	}

	var coeffs [64]float64
	var sorted [64]float64
	idx := 0
	for u := 0; u < videoDCTLowFreqSize; u++ {
		for v := 0; v < videoDCTLowFreqSize; v++ {
			acc := 0.0
			for r := 0; r < videoDCTGridSize; r++ {
				acc += videoDCT2DLowBasis[u][r] * rowDCT[r][v]
			}
			coeffs[idx] = acc
			sorted[idx] = acc
			idx++
		}
	}
	sort.Float64s(sorted[:])
	median := (sorted[31] + sorted[32]) / 2.0

	var bits uint64
	for j, val := range coeffs {
		if val > median {
			bits |= uint64(1) << uint(63-j)
		}
	}
	var out [8]byte
	binary.BigEndian.PutUint64(out[:], bits)
	return out
}

// HashVideoLumaFrame computes the canonical 8-byte DCT perceptual hash of an 8-bit luma frame.
func HashVideoLumaFrame(luma []byte, width, height int) ([8]byte, error) {
	grid, err := DownsampleLuma32x32(luma, width, height)
	if err != nil {
		return [8]byte{}, err
	}
	return dct8x8MedianHash(grid), nil
}

// AudioSpectrogram32x32 builds the 32x32 log-energy time-frequency matrix of mono PCM samples.
func AudioSpectrogram32x32(monoSamples []float64) [videoDCTGridSize][videoDCTGridSize]float64 {
	var matrix [videoDCTGridSize][videoDCTGridSize]float64
	total := len(monoSamples)
	if total == 0 {
		emptyVal := math.Log(videoAudioLogEpsilon)
		for s := 0; s < videoAudioTimeSegments; s++ {
			for b := 0; b < videoAudioFreqBands; b++ {
				matrix[s][b] = emptyVal
			}
		}
		return matrix
	}

	for segIdx := 0; segIdx < videoAudioTimeSegments; segIdx++ {
		start := (segIdx * total) / videoAudioTimeSegments
		end := ((segIdx + 1) * total) / videoAudioTimeSegments
		if end <= start {
			end = start + 1
			if end > total {
				end = total
			}
		}
		segment := monoSamples[start:end]
		segLen := len(segment)

		var starts []int
		if segLen <= videoAudioWindowSize {
			starts = []int{0}
		} else {
			maxStart := segLen - videoAudioWindowSize
			numWin := segLen / videoAudioWindowSize
			if numWin < 1 {
				numWin = 1
			}
			if numWin > videoAudioMaxWindows {
				numWin = videoAudioMaxWindows
			}
			starts = make([]int, numWin)
			if numWin == 1 {
				starts[0] = 0
			} else {
				for w := 0; w < numWin; w++ {
					starts[w] = (w * maxStart) / (numWin - 1)
				}
			}
		}

		var powerSum [videoAudioFreqBands]float64
		var windowed [videoAudioWindowSize]float64
		for _, pos := range starts {
			for n := 0; n < videoAudioWindowSize; n++ {
				sample := 0.0
				if pos+n < segLen {
					sample = segment[pos+n]
				}
				windowed[n] = sample * videoAudioHann[n]
			}
			for band := 0; band < videoAudioFreqBands; band++ {
				proj := 0.0
				for n := 0; n < videoAudioWindowSize; n++ {
					proj += windowed[n] * videoAudioDCTBasis[band][n]
				}
				powerSum[band] += proj * proj
			}
		}
		winCount := float64(len(starts))
		for band := 0; band < videoAudioFreqBands; band++ {
			matrix[segIdx][band] = math.Log(powerSum[band]/winCount + videoAudioLogEpsilon)
		}
	}
	return matrix
}

// HashVideoAudioPCM computes the canonical 8-byte spectrogram perceptual hash of mono PCM samples.
func HashVideoAudioPCM(monoSamples []float64) [8]byte {
	return dct8x8MedianHash(AudioSpectrogram32x32(monoSamples))
}

// HashInterleavedVideoAudioPCMBytes averages little-endian float64 PCM channels per sample frame and hashes the mono signal.
func HashInterleavedVideoAudioPCMBytes(rawPCM []byte, channels int) ([8]byte, error) {
	if channels <= 0 {
		return [8]byte{}, fmt.Errorf("audio channels must be positive, got %d", channels)
	}
	frameByteWidth := channels * 8
	if len(rawPCM)%frameByteWidth != 0 {
		return [8]byte{}, fmt.Errorf("decoded PCM byte length %d is not a multiple of %d (%d channels * 8 bytes)", len(rawPCM), frameByteWidth, channels)
	}
	sampleFrames := len(rawPCM) / frameByteWidth
	mono := make([]float64, sampleFrames)
	invChannels := 1.0 / float64(channels)
	for i := 0; i < sampleFrames; i++ {
		base := i * frameByteWidth
		sum := 0.0
		for ch := 0; ch < channels; ch++ {
			offset := base + ch*8
			sum += math.Float64frombits(binary.LittleEndian.Uint64(rawPCM[offset : offset+8]))
		}
		mono[i] = sum * invChannels
	}
	return HashVideoAudioPCM(mono), nil
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		PixFmt       string `json:"pix_fmt"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		Channels     int    `json:"channels"`
		NBReadFrames string `json:"nb_read_frames"`
	} `json:"streams"`
}

func isProcessTerminatedBySignal(exitErr *exec.ExitError) bool {
	if exitErr == nil || exitErr.ProcessState == nil {
		return false
	}
	if !exitErr.ProcessState.Exited() {
		return true
	}
	exitCode := exitErr.ProcessState.ExitCode()
	// ffmpeg exits with 255 on caught SIGINT/SIGTERM and 123 after >3 caught signals.
	return exitCode == 255 || exitCode == 123
}

func classifyVideoExecError(cmdCtx context.Context, op string, err error) error {
	if cmdCtx.Err() != nil {
		return fmt.Errorf("%s context error: %w", op, cmdCtx.Err())
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if isProcessTerminatedBySignal(exitErr) {
			if stderr != "" {
				return fmt.Errorf("%s terminated by signal (%v): %s", op, err, stderr)
			}
			return fmt.Errorf("%s terminated by signal: %w", op, err)
		}
		if stderr != "" {
			return fmt.Errorf("%w: %s: %s", ErrInvalidUploadedVideo, op, stderr)
		}
		return fmt.Errorf("%w: %s: %w", ErrInvalidUploadedVideo, op, err)
	}
	return fmt.Errorf("%s execution error: %w", op, err)
}

// VerifyAndScoreVideoMP4 validates MP4 container metadata and computes the canonical multi-block VSS score.
func VerifyAndScoreVideoMP4(ctx context.Context, mp4Path string, expectedNumFrames, expectedWidth, expectedHeight int) ([]byte, bool, error) {
	probeCtx, cancelProbe := context.WithTimeout(ctx, 30*time.Second)
	defer cancelProbe()

	probeCmd := exec.CommandContext(
		probeCtx,
		"ffprobe",
		"-v", "error",
		"-count_frames",
		"-print_format", "json",
		"-show_streams",
		mp4Path,
	)
	probeBytes, err := probeCmd.Output()
	if err != nil {
		return nil, false, classifyVideoExecError(probeCtx, "ffprobe", err)
	}
	var probed ffprobeOutput
	if err := json.Unmarshal(probeBytes, &probed); err != nil {
		return nil, false, fmt.Errorf("parse ffprobe output: %w", err)
	}

	hasVideo := false
	hasAudio := false
	var streamWidth, streamHeight, actualFrames, audioChannels int
	for _, s := range probed.Streams {
		if s.CodecType == "video" && !hasVideo {
			if strings.ToLower(s.CodecName) != "h264" {
				return nil, false, fmt.Errorf("%w: unsupported video codec %q, expected h264", ErrInvalidUploadedVideo, s.CodecName)
			}
			if strings.ToLower(s.PixFmt) != "yuv420p" {
				return nil, false, fmt.Errorf("%w: unsupported video pixel format %q, expected yuv420p", ErrInvalidUploadedVideo, s.PixFmt)
			}
			frames, parseErr := strconv.Atoi(strings.TrimSpace(s.NBReadFrames))
			if parseErr != nil || frames <= 0 {
				return nil, false, fmt.Errorf("%w: unable to read positive video frame count", ErrInvalidUploadedVideo)
			}
			hasVideo = true
			streamWidth = s.Width
			streamHeight = s.Height
			actualFrames = frames
		} else if s.CodecType == "audio" && !hasAudio {
			if s.Channels <= 0 {
				return nil, false, fmt.Errorf("%w: invalid audio channel count %d", ErrInvalidUploadedVideo, s.Channels)
			}
			hasAudio = true
			audioChannels = s.Channels
		}
	}
	if !hasVideo {
		return nil, false, fmt.Errorf("%w: uploaded MP4 contains no video stream", ErrInvalidUploadedVideo)
	}
	if actualFrames != expectedNumFrames {
		return nil, false, fmt.Errorf("%w: uploaded video frame count %d does not match expected %d", ErrInvalidUploadedVideo, actualFrames, expectedNumFrames)
	}
	if expectedWidth > 0 && streamWidth != expectedWidth {
		return nil, false, fmt.Errorf("%w: uploaded video width %d does not match expected %d", ErrInvalidUploadedVideo, streamWidth, expectedWidth)
	}
	if expectedHeight > 0 && streamHeight != expectedHeight {
		return nil, false, fmt.Errorf("%w: uploaded video height %d does not match expected %d", ErrInvalidUploadedVideo, streamHeight, expectedHeight)
	}
	if streamWidth <= 0 || streamHeight <= 0 {
		return nil, false, fmt.Errorf("%w: uploaded video has invalid dimensions", ErrInvalidUploadedVideo)
	}

	indices, err := SampledVideoFrameIndices(actualFrames)
	if err != nil {
		return nil, false, err
	}
	uniqueExprs := make([]string, 0, len(indices))
	seenIdx := make(map[int]int, len(indices))
	uniqueOrdered := make([]int, 0, len(indices))
	for _, idx := range indices {
		if _, exists := seenIdx[idx]; !exists {
			seenIdx[idx] = len(uniqueOrdered)
			uniqueOrdered = append(uniqueOrdered, idx)
			uniqueExprs = append(uniqueExprs, fmt.Sprintf("eq(n\\,%d)", idx))
		}
	}
	selectFilter := "select='" + strings.Join(uniqueExprs, "+") + "'"

	decodeCtx, cancelDecode := context.WithTimeout(ctx, 30*time.Second)
	defer cancelDecode()

	// Output raw yuv420p directly so plane 0 preserves the exact H.264 Y plane without libswscale range expansion.
	vCmd := exec.CommandContext(
		decodeCtx,
		"ffmpeg",
		"-v", "error",
		"-i", mp4Path,
		"-map", "0:v:0",
		"-vf", selectFilter,
		"-fps_mode", "passthrough",
		"-f", "rawvideo",
		"-pix_fmt", "yuv420p",
		"pipe:1",
	)
	rawYUV, err := vCmd.Output()
	if err != nil {
		return nil, false, classifyVideoExecError(decodeCtx, "ffmpeg video decode", err)
	}
	yBytes := streamWidth * streamHeight
	uvWidth := (streamWidth + 1) / 2
	uvHeight := (streamHeight + 1) / 2
	frameBytes := yBytes + 2*uvWidth*uvHeight
	if len(rawYUV) != len(uniqueOrdered)*frameBytes {
		return nil, false, fmt.Errorf("%w: decoded yuv420p size %d does not match %d sampled frames", ErrInvalidUploadedVideo, len(rawYUV), len(uniqueOrdered))
	}

	uniqueHashes := make([][8]byte, len(uniqueOrdered))
	for i := range uniqueOrdered {
		offset := i * frameBytes
		block, hashErr := HashVideoLumaFrame(rawYUV[offset:offset+yBytes], streamWidth, streamHeight)
		if hashErr != nil {
			return nil, false, fmt.Errorf("%w: %w", ErrInvalidUploadedVideo, hashErr)
		}
		uniqueHashes[i] = block
	}

	totalBlocks := len(indices)
	if hasAudio {
		totalBlocks++
	}
	score := make([]byte, 0, totalBlocks*8)
	for _, idx := range indices {
		block := uniqueHashes[seenIdx[idx]]
		score = append(score, block[:]...)
	}

	if hasAudio {
		audioCtx, cancelAudio := context.WithTimeout(ctx, 30*time.Second)
		defer cancelAudio()

		// Decode native channel count without -ac 1 so channel averaging matches arithmetic mean instead of libswresample 1/sqrt(2) downmix.
		aCmd := exec.CommandContext(
			audioCtx,
			"ffmpeg",
			"-v", "error",
			"-i", mp4Path,
			"-vn",
			"-map", "0:a:0",
			"-f", "f64le",
			"-acodec", "pcm_f64le",
			"pipe:1",
		)
		rawPCM, aErr := aCmd.Output()
		if aErr != nil {
			return nil, false, classifyVideoExecError(audioCtx, "ffmpeg audio decode", aErr)
		}
		audioBlock, hashErr := HashInterleavedVideoAudioPCMBytes(rawPCM, audioChannels)
		if hashErr != nil {
			return nil, false, fmt.Errorf("%w: %w", ErrInvalidUploadedVideo, hashErr)
		}
		score = append(score, audioBlock[:]...)
	}

	return score, hasAudio, nil
}

// VerifyUploadedVideoScore verifies that the score recomputed from uploaded 0.mp4 matches task.Score.
func VerifyUploadedVideoScore(committedScoreHex string, uploadedScore []byte, hasAudio bool, expectedNumFrames int, maxUploadAudioDistance uint64) error {
	committed, err := hexutil.Decode(committedScoreHex)
	if err != nil {
		return errors.New("invalid committed task score")
	}
	indices, err := SampledVideoFrameIndices(expectedNumFrames)
	if err != nil {
		return err
	}
	visualBytes := len(indices) * 8
	expectedBytes := visualBytes
	if hasAudio {
		expectedBytes += 8
	}
	if len(committed) != expectedBytes || len(uploadedScore) != expectedBytes {
		return fmt.Errorf("%w: uploaded video stream layout does not match committed score length", ErrInvalidUploadedVideo)
	}
	if !bytes.Equal(committed[:visualBytes], uploadedScore[:visualBytes]) {
		return fmt.Errorf("%w: uploaded video visual frames do not match committed score", ErrInvalidUploadedVideo)
	}
	if hasAudio {
		dist := uint64(utils.HammingDistance(committed[visualBytes:expectedBytes], uploadedScore[visualBytes:expectedBytes]))
		if dist > maxUploadAudioDistance {
			return fmt.Errorf("%w: uploaded video audio track does not match committed score", ErrInvalidUploadedVideo)
		}
	}
	return nil
}
