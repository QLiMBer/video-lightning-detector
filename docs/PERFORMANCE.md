# Performance & Measurement

Purpose: clear guidance to measure performance, compare changes, and identify bottlenecks quickly.

## What To Measure
- Wall‑clock time: total runtime and per‑stage durations.
- CPU and memory: where time and allocations go.
- Result stability: detection behavior remains consistent while optimizing.

## Options (complementary)
- CLI logs (quick triage)
  - Total time at Info; per‑stage durations at Debug.
  - Use for fast, coarse bottleneck identification.
- Machine‑readable timings
  - Flag: `--export-timings` writes `timings.json` with per‑stage and total durations (ms) in the output directory.
  - The detector prints a single-line summary `Detections: <N>` at the end of detection for concise counting.
  - Use for automated comparisons and artifact tracking.
- End‑to‑end benchmark (Go test)
  - `main_test.go` reads `VLD_CLI_ARGS` and runs the CLI repeatedly; supports quoted paths.
  - Use `-benchmem` and `-count` for stable numbers and allocation stats.
- CPU/Mem profiling (Go toolchain)
  - `-cpuprofile cpu.prof -memprofile mem.prof` with the benchmark.
  - Inspect with `go tool pprof -text cpu.prof` and `go tool pprof -text mem.prof`. SVGs optional if Graphviz‑enabled pprof is available.
- Microbenchmarks (hotspots)
  - `internal/utils/bench_test.go` covers `ScaleImage` and `BlurImage`.
  - Use to evaluate low‑level changes without full pipeline variance.
- Not a metric: progress bars/spinners
  - Spinners show elapsed time for UX only. Do not use them for measurements or comparisons.

## Quick Baselines
- Per‑stage timing via CLI (skip export to isolate compute)
```
./bin/video-lightning-detector \
  -i resources/samples/sample_yes.mp4 \
  -o ./runs/baseline \
  -a -s 0.4 -f --export-timings --quiet-detections
```
Check `runs/baseline/timings.json`. For detection count prefer `Detections: N`.

- End‑to‑end benchmark with memory stats
```
export VLD_CLI_ARGS='-i resources/samples/sample_yes.mp4 -o ./runs/bench -a -s 0.4 -f'
go test -v -run ^$ -bench BenchmarkVideoLightningDetectorFromEnvArgs -benchmem -count 5
```

- Profiling alongside the benchmark
```
go test -v -run ^$ -bench BenchmarkVideoLightningDetectorFromEnvArgs -cpuprofile cpu.prof -memprofile mem.prof
go tool pprof -text cpu.prof
go tool pprof -text mem.prof
```

- Heavier sample (longer clip)
```
export VLD_CLI_ARGS='-i "resources/samples/sample 1.mp4" -o ./runs/bench-long -a -s 0.4 -f'
go test -v -run ^$ -bench BenchmarkVideoLightningDetectorFromEnvArgs -benchmem
```

- Microbenchmarks only
```
go test -run ^$ -bench '^Benchmark(ScaleImage|BlurImage)' -benchmem -count 10
```

## Convenience Script
Use `scripts/bench.sh` as a wrapper for repeatable local runs.
- Defaults: `VLD_CLI_ARGS='-i resources/samples/sample_yes.mp4 -o runs/bench -a -s 0.4 -f'`
- Customize: `COUNT=7`, `PROFILE=1`, or change `VLD_CLI_ARGS`.
- Example: `COUNT=5 PROFILE=1 ./scripts/bench.sh`

## Perf Results System (runs + compare)
A lightweight file-based system to run suites, store results, and compare against a baseline.

- Setup
```
# Build detector and perf CLI
source ./env.sh
go build -v -o bin/video-lightning-detector .
go build -v -o bin/vld-perf ./cmd/vld-perf

# Configure suites (edit as needed)
cat perf-results/suites.json
```

- Create baseline (per suite)
```
bin/vld-perf run short_pos --label baseline --as-baseline
bin/vld-perf run short_neg --label baseline --as-baseline
# optional long clip
bin/vld-perf run long_pos  --label baseline --as-baseline
```

- Iterate and auto‑compare to baseline
```
bin/vld-perf run short_pos --label opt1
# Helpful flags:
#   --verbose        prints env (VLD_CLI_ARGS) and more context
#   --quiet          suppresses command echo
#   --no-stream      disables streaming detector output to your terminal
# Behavior: vld-perf streams detector output, auto-adds `--quiet-detections`,
#           and prefers the final `Detections: N` summary when present.
```
Shows deltas for: total_ms, analysis_ms, detection_ms, ns/op, B/op, allocs/op.

- Compare historic runs
```
bin/vld-perf compare short_pos baseline 20250905-153012_opt1
```

- List and manage runs
```
# List runs for a suite (baseline marked)
bin/vld-perf list short_pos

# Set a different baseline
bin/vld-perf set-baseline short_pos <run-id>

# Delete a run (file removal)
bin/vld-perf rm short_pos <run-id>
```

- Run IDs and data captured
  - Run ID: `YYYYMMDD-HHMMSS_<label>` (choose a meaningful `--label`).
  - Stored at: `perf-results/<suite>/<run-id>.json` with metadata (commit, branch, go/ffmpeg, OS/arch, suite name, exact CLI args), timings (total + stages), Go bench stats (ns/op, B/op, allocs/op), and detection count (from the `Detections: N` summary; falls back to per-frame positives if needed).
  - Baseline pointer: `perf-results/<suite>/baseline.json`.
## Guardrails for Quality
- Quick: use the single-line `Detections: N` to catch obvious regressions (avoid per-frame logs for totals).
- Deeper: use `-e -j -r` to export CSV/JSON/HTML and compare statistics when needed.

## Next Steps (optional)
- Add microbenches for frame differencing and thresholding.
- Add CLI pprof flags to capture profiles outside `go test`.
- Consider a manual CI bench that uploads `cpu.prof` and `mem.prof` artifacts.
