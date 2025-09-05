package detector

import (
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/Krzysztofz01/video-lightning-detector/internal/utils"
)

type timingsReport struct {
	TotalMs float64            `json:"total_ms"`
	Stages  map[string]float64 `json:"stages_ms"`
}

func writeTimingsJSON(outputDir string, total time.Duration, stages map[string]time.Duration) error {
	report := timingsReport{
		TotalMs: float64(total.Microseconds()) / 1000.0,
		Stages:  make(map[string]float64, len(stages)),
	}
	for k, v := range stages {
		report.Stages[k] = float64(v.Microseconds()) / 1000.0
	}

	dst := path.Join(outputDir, "timings.json")
	f, err := utils.CreateFileWithTree(dst)
	if err != nil {
		return fmt.Errorf("detector: failed to create timings report file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("detector: failed to encode timings report: %w", err)
	}
	return nil
}
