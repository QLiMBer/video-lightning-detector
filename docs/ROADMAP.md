# Roadmap: Performance Tooling & UX Alignment

This document tracks the outstanding work to fully align the codebase with the performance measurement documentation and to polish the developer UX around benchmarking and comparisons.

## Context
- The docs describe a streamlined workflow for performance runs, automated comparisons, and cleaner detector output.
- Some changes landed on branches that were later rebased/merged; a few pieces may be missing or only partially applied in `next`.
- Goal: make the docs an accurate, reliable spec and ensure the code in `next` matches it.

## Status: Completed Items
- Detector UX
  - `--quiet-detections` suppresses per-frame positives and the low-value "Checking frame thresholds." line.
  - Always emits a final single-line summary: `Detections: N`.
  - Wired via `cmd/root.go`, implemented in `internal/detector/detector.go`.

- vld-perf UX
  - Auto-appends `--quiet-detections` for cleaner runs.
  - Streams detector output robustly (no freezes) and shows a bench heartbeat during long `go test -bench` phases.
  - Counts detections via the `Detections: N` summary with fallback to per-frame positives.

- Docs
  - `docs/PERFORMANCE.md` documents quiet detections, the summary line, and vld-perf behavior.
  - README points to PERFORMANCE for performance workflow.

## Remaining / Nice-to-haves
- CI stability
  - Harden ffmpeg setup in CI (retries or cache) to avoid transient fetch failures.

- Test coverage
  - Add light tests where absent (cmd, cmd/vld-perf, internal/render) and basic CLI smoke tests; keep runs fast and deterministic.

## Suggested Next Steps
1) CI tweaks (optional)
   - Add retry wrapper around ffmpeg install; consider caching downloads.
   - Keep format/vet/build/test stages as-is.

## Acceptance Criteria
- DONE: Detector prints a single-line `Detections: N` summary and suppresses per-frame positive logs when `--quiet-detections` is enabled.
- DONE: vld-perf streams output, auto-quiets detections, parses the summary reliably, and keeps responsive during long benches.
- DONE: PERFORMANCE examples run cleanly and produce `timings.json` plus consistent detection counts.
- Pending: CI passes reliably without flaking on ffmpeg setup.

## Branching Plan
- Current work landed on `feature/detector-quiet-and-vld-perf`, targeting `next`.
- Open a Draft PR, ensure CI green, then squash-merge.
