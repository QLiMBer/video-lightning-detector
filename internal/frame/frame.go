package frame

import (
	"image"
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
	// Fast path: operate on *image.RGBA directly in a single fused pass with worker stripes.
	if cur, ok1 := currentFrame.(*image.RGBA); ok1 {
		var prevRGBA *image.RGBA
		if prev, ok2 := previousFrame.(*image.RGBA); ok2 {
			prevRGBA = prev
		}
		return createNewFrameFused(cur, prevRGBA, ordinalNumber)
	}

	// Fallback path for non-RGBA images: preserve previous behavior.
	frame := &Frame{OrdinalNumber: ordinalNumber}
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		frame.Brightness = calculateFrameBrightness(currentFrame)
	}()
	go func() {
		defer wg.Done()
		if ordinalNumber == 1 {
			frame.ColorDifference = 0.0
			return
		}
		frame.ColorDifference = calculateFramesColorDifference(currentFrame, previousFrame)
	}()
	go func() {
		defer wg.Done()
		if ordinalNumber == 1 {
			frame.BinaryThresholdDifference = 0.0
			return
		}
		frame.BinaryThresholdDifference = calculateFramesBinaryThresholdDifference(currentFrame, previousFrame)
	}()
	wg.Wait()
	return frame
}

// createNewFrameFused computes brightness, color diff, and BT diff in a single pass over RGBA data.
func createNewFrameFused(current, previous *image.RGBA, ordinal int) *Frame {
	width := current.Bounds().Dx()
	height := current.Bounds().Dy()
	frameSize := float64(width * height)

	// Accumulators per worker
	type acc struct {
		bSum   float64
		cdSum  float64
		btDiff int
	}

	workers := runtime.GOMAXPROCS(0)
	if workers > height {
		workers = height
	}
	if workers < 1 {
		workers = 1
	}

	accs := make([]acc, workers)
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		w := w
		y0 := (height * w) / workers
		y1 := (height * (w + 1)) / workers
		go func() {
			defer wg.Done()
			cPix := current.Pix
			pPix := []uint8(nil)
			if previous != nil {
				pPix = previous.Pix
			}
			cStride := current.Stride
			pStride := 0
			if previous != nil {
				pStride = previous.Stride
			}
			var bSum, cdSum float64
			btCnt := 0
			for y := y0; y < y1; y++ {
				cRow := y * cStride
				pRow := y * pStride
				for x := 0; x < width; x++ {
					i := cRow + x*4
					r := cPix[i+0]
					g := cPix[i+1]
					b := cPix[i+2]
					bSum += utils.BrightnessFromRGB(r, g, b)
					if previous != nil && ordinal > 1 {
						j := pRow + x*4
						pr := pPix[j+0]
						pg := pPix[j+1]
						pb := pPix[j+2]
						cdSum += utils.ColorDiffRGB(r, g, b, pr, pg, pb)
						// Binary threshold difference without constructing colors
						curWhite := utils.GrayscaleFromRGB(r, g, b) >= BinaryThresholdParam
						prevWhite := utils.GrayscaleFromRGB(pr, pg, pb) >= BinaryThresholdParam
						if curWhite != prevWhite {
							btCnt++
						}
					}
				}
			}
			accs[w] = acc{bSum: bSum, cdSum: cdSum, btDiff: btCnt}
		}()
	}
	wg.Wait()

	// Reduce
	var bSum, cdSum float64
	var btCnt int
	for _, a := range accs {
		bSum += a.bSum
		cdSum += a.cdSum
		btCnt += a.btDiff
	}

	out := &Frame{OrdinalNumber: ordinal}
	out.Brightness = bSum / frameSize
	if ordinal == 1 || previous == nil {
		out.ColorDifference = 0
		out.BinaryThresholdDifference = 0
	} else {
		out.ColorDifference = cdSum / frameSize
		out.BinaryThresholdDifference = float64(btCnt) / frameSize
	}
	return out
}

func calculateFrameBrightness(currentFrame image.Image) float64 {
	// Fallback path using existing helpers; not on performance path.
	sum := 0.0
	bounds := currentFrame.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			sum += utils.GetColorBrightness(currentFrame.At(x, y))
		}
	}
	frameSize := bounds.Dx() * bounds.Dy()
	return sum / float64(frameSize)
}

func calculateFramesColorDifference(currentFrame, previousFrame image.Image) float64 {
	sum := 0.0
	bounds := currentFrame.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			sum += utils.GetColorDifference(currentFrame.At(x, y), previousFrame.At(x, y))
		}
	}
	frameSize := bounds.Dx() * bounds.Dy()
	return sum / float64(frameSize)
}

func calculateFramesBinaryThresholdDifference(currentFrame, previousFrame image.Image) float64 {
	cnt := 0
	bounds := currentFrame.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			g1 := utils.ColorToGrayscale(currentFrame.At(x, y))
			g2 := utils.ColorToGrayscale(previousFrame.At(x, y))
			a := g1 >= BinaryThresholdParam
			b := g2 >= BinaryThresholdParam
			if a != b {
				cnt++
			}
		}
	}
	frameSize := bounds.Dx() * bounds.Dy()
	return float64(cnt) / float64(frameSize)
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
