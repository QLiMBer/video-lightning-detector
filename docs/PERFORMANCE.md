# Performance & Measurement

Purpose: document current benchmarking, timing, and profiling capabilities; provide repeatable commands to capture baselines; outline next steps for deeper optimization work.

## What Exists Today
- Benchmark harness: `main_test.go` defines `BenchmarkVideoLightningDetectorFromEnvArgs` that reads CLI args from the environment variable `VLD_CLI_ARGS` and executes the CLI via `cmd.Execute(...)`.
- Stage timing logs: `internal/detector/detector.go` logs total runtime (Info) and per‑stage durations (Debug) for:
  - Video analysis
  - Auto thresholds
  - Video detection
  - Frames export
- Progress/Spinner with timers: `internal/render/render.go` spinners show elapsed time; progress bars track counts.
- Profiling via `go test` flags: use `-cpuprofile` and `-memprofile` to produce CPU and memory profiles for benchmarks.
- Coverage: standard `go test -coverprofile coverage.out` flow; view with `go tool cover -html`.

## Baseline: Quick Timing (per‑stage)
- See stage durations with verbose logging and skip slow exports to focus on compute cost:
```
./bin/video-lightning-detector \
  -i resources/samples/sample_yes.mp4 \
  -o ./runs/baseline \
  -a -s 0.4 -v -f
```

## Baseline: End‑to‑End Benchmark
1) Set CLI arguments for the benchmark harness (short positive sample, export disabled):
```
export VLD_CLI_ARGS='-i resources/samples/sample_yes.mp4 -o ./runs/bench -a -s 0.4 -f'
```
2) Run benchmarks with memory stats and multiple iterations:
```
go test -v -run ^$ -bench . -benchmem -count 5
```

## Profiling (CPU/Mem)
- Produce profiles while running the benchmark:
```
go test -v -run ^$ -bench . -cpuprofile cpu.prof -memprofile mem.prof
```
- Inspect profiles (examples):
```
go tool pprof -text cpu.prof
go tool pprof -text mem.prof
```
- Optional SVGs (requires Graphviz-enabled pprof):
```
go tool pprof -svg -output cpu-profile.svg cpu.prof
go tool pprof -svg -output mem-profile.svg mem.prof
```

## Heavier Sample (longer video)
- Use the bundled longer clip to stress the pipeline:
```
export VLD_CLI_ARGS='-i "resources/samples/sample 1.mp4" -o ./runs/bench-long -a -s 0.4 -f'
go test -v -run ^$ -bench . -benchmem
```

## Gaps and Next Steps
- Microbenchmarks: add focused benches for hotspots (`utils.ScaleImage`, per‑frame differencing, blur/thresholding) to evaluate algorithmic or SIMD changes.
- CLI pprof flags (optional): add `--profile-cpu` / `--profile-mem` so profiles can be captured outside `go test`.
- Timing summary export: write a concise `timings.json` to the output directory with per‑stage and total durations for automated tracking.
- CI/nightly bench (optional): a manual workflow that sets `VLD_CLI_ARGS`, runs benches and uploads `cpu.prof`/`mem.prof` artifacts.

## Suggested Branch for Follow‑up
- `feature/benchmarks-baseline` to add microbenches and (optionally) timing summary export.

