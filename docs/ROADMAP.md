# Roadmap: Performance Tooling & UX Alignment

This document tracks the outstanding work to fully align the codebase with the performance measurement documentation and to polish the developer UX around benchmarking and comparisons.

## Context
- The docs describe a streamlined workflow for performance runs, automated comparisons, and cleaner detector output.
- Some changes landed on branches that were later rebased/merged; a few pieces may be missing or only partially applied in `next`.
- Goal: make the docs an accurate, reliable spec and ensure the code in `next` matches it.

## Leftovers To Implement/Verify
- Quiet detections and summary line
  - Add/verify detector option `QuietDetections` to suppress per-frame positives.
  - Emit a final single-line summary at the end of detection: `Detections: <N>`.
  - Register CLI flag `--quiet-detections` (root command) and wire to detector options.

- vld-perf behavior
  - Auto-append `--quiet-detections` to the detector CLI unless explicitly disabled.
  - Stream detector output live; avoid forcing `-v` (silence low-value logs by default).
  - Prefer parsing the final `Detections: N` summary; fallback to counting per-frame positives only if summary absent.
  - Use chunked stdout reading (not line-scanner) to avoid freezes with progress bars/spinners.
  - Add a lightweight heartbeat during `go test -bench` work to show liveness while compiling/running.

- Docs alignment
  - Ensure README and PERFORMANCE reference `--export-timings`, `--quiet-detections`, and the single-line `Detections: N` behavior.
  - Confirm example commands in PERFORMANCE match actual flags/paths and work out of the box.

- CI stability
  - Harden ffmpeg setup in CI (add retries or cache) to avoid transient fetch failures.

## Suggested Next Steps
1) Detector alignment
   - Add/confirm `QuietDetections` in `internal/detector/options.go`.
   - Gate per-frame positive logs on `QuietDetections`.
   - Emit `Detections: %d` once at the end of the run.
   - Wire CLI flag `--quiet-detections` in `cmd/root.go`.

2) vld-perf polish
   - Ensure command echo + live streaming with chunked read (no Scanner token limits).
   - Default to adding `--quiet-detections` (allow override via `--no-auto-quiet`).
   - Parse `Detections: N`; fallback to per-frame lines if needed.
   - Bench heartbeat during `go test` compile/run phases.
   - Do not force `-v`; rely on progress/summary for feedback.

3) Docs sync
   - Keep `docs/PERFORMANCE.md` as the spec; update examples after code changes are merged.
   - Short usage notes in README pointing to PERFORMANCE for details.

4) CI tweaks (optional)
   - Add retry wrapper around ffmpeg install; consider caching downloads.
   - Keep format/vet/build/test stages as-is.

## Acceptance Criteria
- Detector prints a single-line `Detections: N` summary and suppresses per-frame positive logs when `--quiet-detections` is enabled.
- vld-perf streams output, auto-quiets detections, parses the summary reliably, and keeps responsive during long benches.
- PERFORMANCE examples run cleanly on `next` without edits and produce `timings.json` plus consistent detection counts.
- CI passes reliably without flaking on ffmpeg setup.

## Branching Plan
- Use `feature/perf-ux-alignment` for the next pass.
- Open PR into `next`; keep diffs focused; run fmt/vet/build/test before pushing.
- After merge, prune the feature branch locally (`git branch -D feature/perf-ux-alignment`) and remotely if created.

