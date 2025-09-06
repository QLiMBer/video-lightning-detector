package frame

import (
	"image"
	"image/color"
	"runtime"
	"strconv"
	"sync"

	"github.com/Krzysztofz01/video-lightning-detector/internal/utils"
)

const (
	// TODO: Implement different threshold for day/night. The brightness value can be used as the determinant.
	BinaryThresholdParam float64 = 0.784313725
)

// Strucutre representing a single video frame and its calculated parameters.
// TODO: When it coms to BinaryThreshold we need to test which approach gives better results.
// Currently we are comparing the BT of the previous and current frame and than calcualte the white_pixels / all_pixels
// Alternatively we can just count the occurance of white pixels and return the non-normalized result
type Frame struct {
	OrdinalNumber             int     `json:"ordinal-number"`
	ColorDifference           float64 `json:"color-difference"`
	BinaryThresholdDifference float64 `json:"binary-threshold-difference"`
	Brightness                float64 `json:"brightness"`
}

// Create a new frame instance by providing the current and previous frame images and the ordinal number of the frame.
func CreateNewFrame(currentFrame, previousFrame image.Image, ordinalNumber int) *Frame {
	frame := &Frame{
		OrdinalNumber: ordinalNumber,
	}

	// Prefer a fused single-pass compute on RGBA for performance; fallback to the legacy path otherwise.
	if cur, ok := currentFrame.(*image.RGBA); ok {
		var prev *image.RGBA
		if ordinalNumber > 1 {
			if p, ok2 := previousFrame.(*image.RGBA); ok2 {
				prev = p
			} else {
				// Fallback when previous is not RGBA.
				goto legacy
			}
		}

		br, cd, bt := computeFrameMetricsRGBA(cur, prev)
		frame.Brightness = br
		frame.ColorDifference = cd
		frame.BinaryThresholdDifference = bt
		return frame
	}

legacy:
	// Legacy path: preserve behavior using simple sequential loops.
	frame.Brightness = calculateFrameBrightnessLegacy(currentFrame)
	if ordinalNumber == 1 {
		frame.ColorDifference = 0.0
		frame.BinaryThresholdDifference = 0.0
	} else {
		frame.ColorDifference = calculateFramesColorDifferenceLegacy(currentFrame, previousFrame)
		frame.BinaryThresholdDifference = calculateFramesBinaryThresholdDifferenceLegacy(currentFrame, previousFrame)
	}
	return frame
}

// Fused single-pass compute over RGBA with row/stripe workers and per-worker accumulators.
// prev may be nil (for the first frame), in which case diffs are zero.
func computeFrameMetricsRGBA(cur *image.RGBA, prev *image.RGBA) (brightness float64, colorDiff float64, btDiff float64) {
	b := cur.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 0, 0, 0
	}

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > h {
		workers = h
	}

	type part struct {
		br, cd float64
		bt     int
	}
	parts := make([]part, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		y0 := (h * i) / workers
		y1 := (h * (i + 1)) / workers
		go func() {
			defer wg.Done()
			stride := cur.Stride
			var pStride int
			if prev != nil {
				pStride = prev.Stride
			}
			local := part{}
			for y := y0; y < y1; y++ {
				off := y*stride + 0
				var pOff int
				if prev != nil {
					pOff = y*pStride + 0
				}
				for x := 0; x < w; x++ {
					r := cur.Pix[off+0]
					g := cur.Pix[off+1]
					b := cur.Pix[off+2]
					// Brightness: match utils.GetColorBrightness by passing color.RGBA
					local.br += utils.GetColorBrightness(color.RGBA{R: r, G: g, B: b, A: 255})

					if prev != nil {
						pr := prev.Pix[pOff+0]
						pg := prev.Pix[pOff+1]
						pb := prev.Pix[pOff+2]
						// Color diff
						local.cd += utils.GetColorDifference(
							color.RGBA{R: r, G: g, B: b, A: 255},
							color.RGBA{R: pr, G: pg, B: pb, A: 255},
						)
						// Binary threshold difference
						if utils.BinaryThreshold(color.RGBA{R: r, G: g, B: b, A: 255}, BinaryThresholdParam) !=
							utils.BinaryThreshold(color.RGBA{R: pr, G: pg, B: pb, A: 255}, BinaryThresholdParam) {
							local.bt++
						}
					}
					off += 4
					if prev != nil {
						pOff += 4
					}
				}
			}
			parts[i] = local
		}()
	}
	wg.Wait()

	var brSum, cdSum float64
	var btSum int
	for _, p := range parts {
		brSum += p.br
		cdSum += p.cd
		btSum += p.bt
	}
	size := float64(w * h)
	brightness = brSum / size
	if prev != nil {
		colorDiff = cdSum / size
		btDiff = float64(btSum) / size
	} else {
		colorDiff = 0
		btDiff = 0
	}
	return
}

// Legacy sequential implementations used when images are not RGBA.
func calculateFrameBrightnessLegacy(currentFrame image.Image) float64 {
	b := currentFrame.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 0
	}
	sum := 0.0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sum += utils.GetColorBrightness(currentFrame.At(x, y))
		}
	}
	return sum / float64(w*h)
}

func calculateFramesColorDifferenceLegacy(currentFrame, previousFrame image.Image) float64 {
	b := currentFrame.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 0
	}
	sum := 0.0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			sum += utils.GetColorDifference(currentFrame.At(x, y), previousFrame.At(x, y))
		}
	}
	return sum / float64(w*h)
}

func calculateFramesBinaryThresholdDifferenceLegacy(currentFrame, previousFrame image.Image) float64 {
	b := currentFrame.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 0
	}
	diff := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if utils.BinaryThreshold(currentFrame.At(x, y), BinaryThresholdParam) !=
				utils.BinaryThreshold(previousFrame.At(x, y), BinaryThresholdParam) {
				diff++
			}
		}
	}
	return float64(diff) / float64(w*h)
}

// Convert the frame string buffer format accepted by the CSV encoder.
func (frame *Frame) ToBuffer() []string {
	buffer := make([]string, 0, 3)
	buffer = append(buffer, strconv.Itoa(frame.OrdinalNumber))
	buffer = append(buffer, strconv.FormatFloat(frame.Brightness, 'f', -1, 64))
	buffer = append(buffer, strconv.FormatFloat(frame.ColorDifference, 'f', -1, 64))
	buffer = append(buffer, strconv.FormatFloat(frame.BinaryThresholdDifference, 'f', -1, 64))

	return buffer
}
