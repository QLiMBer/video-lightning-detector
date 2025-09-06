package frame

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Structure representing the collection of video frames.
// Optimized for ordered access while preserving existing semantics:
// - Allows out-of-order Append (by ordinal) but expects no gaps when exporting GetAll (same as previous behavior).
type FramesCollection struct {
	frames                     []*Frame
	count                      int
	cachedStatisticsValue      *FramesStatistics
	cachedStatisticsResolution int
}

// Create a new frames collection with a given capacity of frames.
func CreateNewFramesCollection(frames int) *FramesCollection {
	return &FramesCollection{
		frames:                     make([]*Frame, frames),
		count:                      0,
		cachedStatisticsValue:      nil,
		cachedStatisticsResolution: 0,
	}
}

// Add a new frame to the frames collection.
func (fc *FramesCollection) Append(frame *Frame) error {
	if frame == nil {
		return errors.New("frame: can not appenda nil frame to the frames collection")
	}
	idx := frame.OrdinalNumber - 1
	if idx < 0 || idx >= len(fc.frames) {
		return errors.New("frame: frame ordinal number out of bounds")
	}
	if fc.frames[idx] != nil {
		return errors.New("frame: frame with a given ordinal number already exists")
	}
	fc.frames[idx] = frame
	fc.count++
	fc.cachedStatisticsValue = nil
	fc.cachedStatisticsResolution = 0
	return nil
}

// Get a frame from the frames collection by the frame ordinal number.
func (fc *FramesCollection) Get(frameNumber int) (*Frame, error) {
	idx := frameNumber - 1
	if idx < 0 || idx >= len(fc.frames) || fc.frames[idx] == nil {
		return nil, errors.New("frame: frame with a given ordinal number does not exist")
	}
	return fc.frames[idx], nil
}

// Get all frames sorted by the frame ordinal number.
// Preserves previous invariant: expects contiguous frames from 1..count; panics if a gap is detected.
func (fc *FramesCollection) GetAll() []*Frame {
	values := make([]*Frame, fc.count)
	for i := 0; i < fc.count; i++ {
		f := fc.frames[i]
		if f == nil {
			panic("frame: missing frame spotted during frames iteration")
		}
		values[i] = f
	}
	return values
}

// Calculate the descriptive statistics values for the given frames collection.
func (fc *FramesCollection) CalculateStatistics(movingMeanResolution int) FramesStatistics {
	if fc.cachedStatisticsValue == nil || fc.cachedStatisticsResolution != movingMeanResolution {
		fc.cachedStatisticsValue = CreateNewFramesStatistics(fc.GetAll(), movingMeanResolution)
		fc.cachedStatisticsResolution = movingMeanResolution
	}
	return *fc.cachedStatisticsValue
}

// Write the JSON format frames report to the provided writer which can be a file reference.
func (fc *FramesCollection) ExportJsonReport(file io.Writer) error {
	framesSlice := fc.GetAll()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")

	if err := encoder.Encode(framesSlice); err != nil {
		return fmt.Errorf("frame: failed to encode the frames collection to json report file: %w", err)
	}

	return nil
}

// Write the CSV format frames report to the provided writer which can be a file reference.
func (fc *FramesCollection) ExportCsvReport(file io.Writer) error {
	framesSlice := fc.GetAll()

	csvWriter := csv.NewWriter(file)
	if err := csvWriter.Write([]string{"Frame", "Brightness", "ColorDifference", "BinaryThresholdDifference"}); err != nil {
		return fmt.Errorf("frame: failed to write the header to the frames report file: %w", err)
	}

	for _, frame := range framesSlice {
		if err := csvWriter.Write(frame.ToBuffer()); err != nil {
			return fmt.Errorf("frame: failed to write the frame to the frames report file: %w", err)
		}
	}

	csvWriter.Flush()
	return nil
}
