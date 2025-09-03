# Project Roadmap

Purpose: track near-term improvements and future ideas. Keep items small and actionable; link PRs when delivered.

## Near-Term
- Add a negative sample video with no lightning under `resources/samples/` (tiny, downscaled) for functional tests.
- Add functional tests that validate detection outcomes:
  - Positive case: ensure at least one detection on a known lightning clip.
  - Negative case: ensure zero detections on the no-lightning sample.
- Add a lightweight bench workflow (manual trigger) using `VLD_CLI_ARGS` to profile CPU/MEM and upload artifacts.
- Consider enabling `golangci-lint` with a fast preset to complement `go vet`.

## Later
- Release automation: tag-driven releases with changelog (Release Please) and/or multi-OS binaries (GoReleaser).
- Improve CI caching and matrix (Go versions) once the pipeline is stable.
- Optional: tiny preview artifact (few frames or chart) for smoke step if runtime remains acceptable.

## Done
- CI smoke test on `resources/samples/sample 0.mp4` to verify execution paths (does not assert detections yet).

