# Project Roadmap

Purpose: track near-term improvements and future ideas. Keep items small and actionable; link PRs when delivered.

## Near-Term
- Add a lightweight bench workflow (manual trigger) using `VLD_CLI_ARGS` to profile CPU/MEM and upload artifacts.
- Consider enabling `golangci-lint` with a fast preset to complement `go vet`.
 - See Performance & Measurement guide: docs/PERFORMANCE.md

## Later
- Release automation: tag-driven releases with changelog (Release Please) and/or multi-OS binaries (GoReleaser).
- Improve CI caching and matrix (Go versions) once the pipeline is stable.
- Optional: tiny preview artifact (few frames or chart) for smoke step if runtime remains acceptable.

## Done
- CI functional assertions on tiny samples (`sample_yes.mp4` >0 detections; `sample_no.mp4` == 0). Removed separate smoke step.
- Added negative and positive bundled samples.
