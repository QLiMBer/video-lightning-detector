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
  - `--no-ui` disables progress bars, spinners, and tables to reduce terminal overhead during perf runs.

- vld-perf UX
  - Auto-appends `--quiet-detections` for cleaner runs.
  - Auto-adds `--no-ui` to further stabilize timings.
  - Streams detector output robustly (no freezes) and shows a bench heartbeat during long `go test -bench` phases.
  - Counts detections via the `Detections: N` summary with fallback to per-frame positives.
  - Compare output now shows `detections` and highlights changes; ANSI codes stripped before parsing to ensure accurate counts.

- Docs
  - `docs/PERFORMANCE.md` documents quiet detections, the summary line, and vld-perf behavior.
  - Performance docs and README updated with `--no-ui` usage and compare behavior.
  - README points to PERFORMANCE for performance workflow.

- Pipeline optimizations
  - FramesCollection refactored from map+locks to slice (behavior preserved).
  - Fused per-frame compute (single RGBA pass with per-row workers) implemented for brightness, color diff, and BT diff, preserving detection results.

## Remaining / Nice-to-haves
- CI stability
  - Harden ffmpeg setup in CI (retries or cache) to avoid transient fetch failures.

- Test coverage
  - Add light tests where absent (cmd, cmd/vld-perf, internal/render) and basic CLI smoke tests; keep runs fast and deterministic.

- Optional: Microbenchmarks for hotspots
  - Rationale: isolate inner-loop changes (e.g., fused per-frame compute, RGBA byte access) from end-to-end noise (decode, I/O, UI). They run in milliseconds and provide stable allocs/op and ns/op for just the targeted function.
  - Scope: keep very small — 1–2 benches under `internal/frame` or `internal/utils` (e.g., fused compute, scaling). Do not replace vld-perf; use as a complement when iterating on tight loops.
  - Decision: defer unless we hit noisy or ambiguous vld-perf results during optimization.

## Suggested Next Steps
1) CI tweaks (optional)
   - Add retry wrapper around ffmpeg install; consider caching downloads.
   - Keep format/vet/build/test stages as-is.

## Acceptance Criteria
- DONE: Detector prints a single-line `Detections: N` summary and suppresses per-frame positive logs when `--quiet-detections` is enabled.
- DONE: `--no-ui` flag available; vld-perf auto-applies it.
- DONE: vld-perf streams output, auto-quiets detections, parses the summary reliably, and keeps responsive during long benches.
- DONE: vld-perf compare shows detections count and strips ANSI before parsing.
- DONE: PERFORMANCE examples run cleanly and produce `timings.json` plus consistent detection counts.
- DONE: Fused compute and slice collection land with preserved behavior and measurable improvements.
- Pending: CI passes reliably without flaking on ffmpeg setup.

## Branching Plan
- Current work landed on `feature/detector-quiet-and-vld-perf`, targeting `next`.
- Open a Draft PR, ensure CI green, then squash-merge.
