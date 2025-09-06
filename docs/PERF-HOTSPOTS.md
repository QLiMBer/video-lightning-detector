# Perf Hotspots & Optimization Plan

Purpose: capture current performance findings and a sequenced, high‑impact plan to optimize the video lightning detector. This complements docs/PERFORMANCE.md (measurement/how) with concrete what/where to optimize.

## Summary
- Pipeline hotspots are CPU and memory‑bandwidth bound in the analysis stage.
- The current implementation does three full per‑pixel passes per frame and uses atomics inside per‑pixel callbacks, magnifying overhead.
- Additional overhead comes from map+locks for frame storage and UI rendering during runs.
- Optional denoise is expensive due to extra allocations and copies.

## Major Findings (Ordered by Impact)
- Atomics in inner pixel loops: calculateFrameBrightness, calculateFramesColorDifference, calculateFramesBinaryThresholdDifference use go.uber.org/atomic within `pimit.ParallelRead` callbacks. This causes heavy contention and synchronization overhead at pixel granularity.
- Triple full‑frame traversals per frame: brightness, color diff, and binary threshold diff each scan the image separately. This multiplies memory traffic and callback overhead by ~3×.
- Interface/color conversions on hot path: using `color.Color` and `Image.At` triggers interface dispatch and conversions (`ColorToRgba`, RGBA shifts). For `*image.RGBA` inputs we can read bytes directly.
- FramesCollection storage: uses `map[int]*Frame` + `sync.RWMutex` for a strictly sequential set [1..N]. Hashing, locking, and pointer chasing add avoidable overhead; a slice is sufficient.
- Console rendering during runs: progress increments per frame and per‑frame Info logs (unless `--quiet-detections`) cost I/O and add variance. vld-perf already suppresses some logs but not progress bars/spinners.
- Denoise path is heavy: `stackblur-go` returns a new image, we convert then copy (`draw.Draw` + `copy`), per frame. If enabled, it can dominate time.
- Frame export re-decodes: exporting reopens the video and random‑seeks (`ReadFrames`), causing repeated decodes from keyframes. Not used during vld-perf (we force `-f`), but relevant for real runs.

## Notable Code Spots
- internal/frame/frame.go
  - `calculateFrameBrightness`, `calculateFramesColorDifference`, `calculateFramesBinaryThresholdDifference`: per‑pixel callbacks with atomics; three separate passes; use of `Image.At` and `color.Color`.
  - `CreateNewFrame` spawns 3 goroutines per frame while the callee functions also parallelize.
- internal/detector/detector.go
  - `performVideoAnalysis`: per‑frame progress increments; debug/info logs; minor error‑check bug on scaling (see below).
  - `applyAutoThresholds`: logic compares defaults to themselves, so user‑provided thresholds are overwritten (correctness, not perf).
- internal/frame/collection.go
  - `FramesCollection` is a map with RWMutex; `GetAll` rebuilds a slice by sequential map lookups.
- internal/utils/image.go
  - `BlurImage` allocates intermediate RGBA and copies every call.

## Library/Framework Notes
- Vidio (decode): reasonable; main cost is decode + copy to RGBA. Hardware decode would be a larger project.
- pimit (parallel read): convenient but encourages per‑pixel callbacks; better to switch to row/stripe workers with per‑worker accumulators.
- x/image/draw: using NearestNeighbor is good for speed.
- go-echarts: only heavy when explicitly exporting charts.
- pterm: great UX; for perf runs we should offer a no‑UI renderer.

## Quick Wins & Bug Fixes
- Fix error check in `performVideoAnalysis`:
  - Replace `if utils.ScaleImage(...); err != nil` with `if err := utils.ScaleImage(...); err != nil { ... }`.
- Respect explicit thresholds in `applyAutoThresholds`:
  - Compare `detector.options.*` against defaults, not defaults vs defaults.

## Proposed Implementation Plan (High → Medium Impact)
Each step should be its own focused PR, targeting `next`. Use vld-perf to create a run for the step and compare to baseline.

Status: Completed steps are marked accordingly.

1) Fuse per‑frame pixel passes; remove atomics (Highest Impact) — COMPLETED
- What: In one image traversal per frame, compute brightness, color diff (needs previous frame), and binary threshold diff. Use row/stripe workers (<= NumCPU) with per‑worker accumulators; reduce at the end. Operate directly on `*image.RGBA` bytes to avoid interface overhead.
- Changes: internal/frame/frame.go (core compute), remove `pimit` from hot path, drop `atomic` in loops; keep API the same.
- Validation: microbench for fused loop; vld-perf run short_pos and short_neg; compare analysis_ms and ns/op.
- Risk: medium (central logic rewrite). Mitigation: keep unit tests; add equality checks vs previous results on sample clips.

2) Switch FramesCollection map+locks to slice (High Impact, low risk) — COMPLETED
- What: Store frames in a `[]Frame` (or `[]*Frame`), append in order, drop locks. Make `GetAll()` return the slice. Cache statistics as today.
- Changes: internal/frame/collection.go and call sites.
- Validation: unit tests + vld-perf analysis_ms improvements; less allocations.
- Risk: low.

3) Add no‑UI renderer and `--no-ui` flag (Stability/Variance win) — COMPLETED
- What: Implement a renderer that is a no‑op for progress, spinner, and table; minimal Info logging. vld-perf should pass `--no-ui` automatically.
- Changes: internal/render (new implementation), cmd/root.go flag, cmd/vld-perf wiring.
- Validation: vld-perf runs with lower variance and slightly lower total_ms.
- Risk: low.

4) O(n) moving mean via sliding window (Medium Impact when window grows)
- What: Replace `utils.MovingMean` (O(window) per index) with a sliding window using prefix sums or deque for constant‑time updates.
- Changes: internal/utils/math.go and users in statistics.
- Validation: microbench + check reports unchanged.
- Risk: low.

5) Denoise fast path (Optional, situational)
- What: Provide a lightweight in‑place box blur for RGBA with small radius; minimize allocations. Keep `stackblur` as quality option.
- Changes: internal/utils/image.go (new fast function; flag `--denoise-fast`), detector wiring.
- Validation: microbench and A/B detection stability on noisy clips.
- Risk: medium (quality trade‑offs).

6) Minor fixes and cleanup (Low Impact)
- Apply the two bug fixes (scale error check; thresholds overwrite).
- Avoid per‑frame goroutine churn in `CreateNewFrame` once the fused parallel loop lands.
- Consider reducing log noise further under `--quiet-detections`.

## Measurement Protocol per Step
- Build: `go build -v -o bin/video-lightning-detector . && go build -v -o bin/vld-perf ./cmd/vld-perf`
- Baseline (already set): use `bin/vld-perf run <suite> --label baseline --as-baseline` once per suite.
- After each PR: `bin/vld-perf run short_pos --label stepN` and `bin/vld-perf run short_neg --label stepN` then `bin/vld-perf compare short_pos baseline <run-id>` (and similarly for short_neg). Track: total_ms, analysis_ms, detection_ms, ns/op, B/op, allocs/op.

## Expected Impact (Ballpark)
- Step 1 (fused, no atomics, RGBA access): 2×–5× faster analysis stage; large drop in allocations and GC pressure.
- Step 2 (slice collection): small but measurable reduction in CPU and allocations.
- Step 3 (no‑UI): lower variance and small wall‑time improvements.
- Step 4 (O(n) moving mean): improves report generation and auto‑threshold stages for large windows.
- Step 5 (denoise fast): optional speedup when denoise is needed.

## Risks & Rollback
- Any detection changes must be justified; keep sample outputs and use `Detections: N` guardrail.
- Each step is isolated; rollback by reverting the corresponding commit/PR.

## Next Actions
- Branch: `feature/perf-hotspots`.
- Completed: Steps 1–3 (fused compute, slice collection, no‑UI + vld-perf updates).
- Next: Step 4 (O(n) moving mean), then optional Step 5 (denoise fast). Consider minimal microbenches only if vld-perf results get noisy.

## Results Snapshot (vs initial baseline)
- short_pos
  - total_ms: 5171 -> 3117 (−39.7%)
  - analysis_ms: 5167 -> 3117 (−39.7%)
  - detection_ms: 2 -> 0 (−93.8%)
  - ns/op: 5375177660 -> 2122344051 (−60.5%)
  - B/op: 15096 -> 15424 (+2.2%)
  - allocs/op: 47 -> 50 (+6.4%)
  - detections: 6 -> 6

- short_neg
  - total_ms: 2385 -> 852 (−64.3%)
  - analysis_ms: 2383 -> 852 (−64.3%)
  - detection_ms: 1 -> 0 (−93.0%)
  - ns/op: 2436056230 -> 882390152 (−63.8%)
  - B/op: 15240 -> 9632 (−36.8%)
  - allocs/op: 48 -> 27 (−43.8%)
  - detections: 0 -> 0

- long_pos
  - total_ms: 88776 -> 31866 (−64.1%)
  - analysis_ms: 88748 -> 31865 (−64.1%)
  - detection_ms: 26 -> 1 (−96.9%)
  - ns/op: 89660147051 -> 32734611460 (−63.5%)
  - B/op: 15088 -> 15088 (+0.0%)
  - allocs/op: 48 -> 48 (+0.0%)
  - detections: 23 -> 23
