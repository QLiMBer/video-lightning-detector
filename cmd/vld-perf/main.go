package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type suitesConfig map[string]string

type timingsReport struct {
	TotalMs float64            `json:"total_ms"`
	Stages  map[string]float64 `json:"stages_ms"`
}

type runMetadata struct {
	RunID        string `json:"run_id"`
	Suite        string `json:"suite"`
	Label        string `json:"label"`
	CommitSHA    string `json:"commit_sha"`
	Branch       string `json:"branch"`
	GoVersion    string `json:"go_version"`
	Ffmpeg       string `json:"ffmpeg_version,omitempty"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	CLIArgs      string `json:"cli_args"`
	TimestampISO string `json:"timestamp_iso"`
}

type benchStats struct {
	NsPerOp     float64 `json:"ns_per_op"`
	BytesPerOp  float64 `json:"bytes_per_op"`
	AllocsPerOp float64 `json:"allocs_per_op"`
}

type runResult struct {
	Metadata   runMetadata   `json:"metadata"`
	Timings    timingsReport `json:"timings_ms"`
	Bench      benchStats    `json:"bench"`
	Detections int           `json:"detections"`
	Notes      string        `json:"notes,omitempty"`
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  vld-perf run <suite> [--label <label>] [--as-baseline] [--threshold <pct>] [--verbose] [--quiet] [--no-stream]")
	fmt.Println("  vld-perf compare <suite> <lhs-run-id|baseline> <rhs-run-id>")
	fmt.Println("  vld-perf list [<suite>]")
	fmt.Println("  vld-perf set-baseline <suite> <run-id>")
	fmt.Println("  vld-perf rm <suite> <run-id>")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	switch cmd {
	case "run":
		runCmd(os.Args[2:])
	case "compare":
		compareCmd(os.Args[2:])
	case "list":
		listCmd(os.Args[2:])
	case "set-baseline":
		setBaselineCmd(os.Args[2:])
	case "rm":
		rmCmd(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

type runOpts struct {
	echo      bool
	verbose   bool
	stream    bool
	threshold float64
}

func runCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "run requires <suite>")
		os.Exit(1)
	}
	suite := args[0]
	label := "run"
	asBaseline := false
	threshold := 5.0
	quiet := false
	verbose := false
	noStream := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--label":
			if i+1 >= len(args) {
				fatalf("--label requires a value")
			}
			label = args[i+1]
			i++
		case "--as-baseline":
			asBaseline = true
		case "--threshold":
			if i+1 >= len(args) {
				fatalf("--threshold requires a value")
			}
			fmt.Sscanf(args[i+1], "%f", &threshold)
			i++
		case "--quiet":
			quiet = true
		case "--verbose":
			verbose = true
		case "--no-stream":
			noStream = true
		default:
			fatalf("unknown flag: %s", args[i])
		}
	}

	suites := mustLoadSuites()
	cli, ok := suites[suite]
	if !ok {
		fatalf("suite not found: %s", suite)
	}

	runID := time.Now().Format("20060102-150405") + "_" + sanitize(label)
	opts := runOpts{echo: !quiet, verbose: verbose, stream: !noStream, threshold: threshold}

	// Echo planned commands
	if opts.echo {
		fmt.Printf("bench> go test -v -run ^$ -bench BenchmarkVideoLightningDetectorFromEnvArgs -benchmem -count 1\n")
		if opts.verbose {
			fmt.Printf("env> VLD_CLI_ARGS=%s\n", cli)
		}
	}

	res := mustExecuteSuite(suite, runID, label, cli, opts)
	mustWriteRun(suite, runID, res)
	fmt.Printf("Saved: perf-results/%s/%s.json\n", suite, runID)

	// Compare to baseline if exists
	baseID, hadBaseline := loadBaselineID(suite)
	if hadBaseline {
		lhs := mustReadRun(suite, baseID)
		printComparison("baseline", lhs, runID, res, threshold)
	}

	if asBaseline {
		mustWriteBaselineID(suite, runID)
		if hadBaseline {
			fmt.Printf("Baseline set to %s\n", runID)
		} else {
			fmt.Printf("Baseline initialized: %s\n", runID)
		}
	} else if !hadBaseline {
		fmt.Println("No baseline set; use set-baseline or --as-baseline to define one.")
	}
}

func compareCmd(args []string) {
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, "compare requires <suite> <lhs-run-id|baseline> <rhs-run-id>")
		os.Exit(1)
	}
	suite := args[0]
	lhsID := args[1]
	rhsID := args[2]
	if lhsID == "baseline" {
		id, ok := loadBaselineID(suite)
		if !ok {
			fatalf("no baseline set for suite %s", suite)
		}
		lhsID = id
	}
	lhs := mustReadRun(suite, lhsID)
	rhs := mustReadRun(suite, rhsID)
	printComparison(lhs.Metadata.RunID, lhs, rhs.Metadata.RunID, rhs, 5.0)
}

func listCmd(args []string) {
	suite := ""
	if len(args) >= 1 {
		suite = args[0]
	}
	root := filepath.Join("perf-results")
	if suite == "" {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			if e.IsDir() {
				listSuite(e.Name())
			}
		}
		return
	}
	listSuite(suite)
}

func listSuite(suite string) {
	dir := filepath.Join("perf-results", suite)
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read suite %s: %v\n", suite, err)
		return
	}
	baseID, _ := loadBaselineID(suite)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == "baseline.json" {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		res := mustReadRun(suite, id)
		mark := ""
		if id == baseID {
			mark = " *baseline"
		}
		fmt.Printf("%s%s  total=%.0fms  ns/op=%.0f  allocs/op=%.0f\n", id, mark, res.Timings.TotalMs, res.Bench.NsPerOp, res.Bench.AllocsPerOp)
	}
}

func setBaselineCmd(args []string) {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "set-baseline requires <suite> <run-id>")
		os.Exit(1)
	}
	suite := args[0]
	runID := args[1]
	mustWriteBaselineID(suite, runID)
	fmt.Printf("Baseline set to %s\n", runID)
}

func rmCmd(args []string) {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "rm requires <suite> <run-id>")
		os.Exit(1)
	}
	suite := args[0]
	runID := args[1]
	p := filepath.Join("perf-results", suite, runID+".json")
	if err := os.Remove(p); err != nil {
		fatalf("failed to remove %s: %v", p, err)
	}
	fmt.Printf("Removed %s\n", p)
}

func mustLoadSuites() suitesConfig {
	p := filepath.Join("perf-results", "suites.json")
	b, err := os.ReadFile(p)
	if err != nil {
		fatalf("failed to read suites.json: %v", err)
	}
	var s suitesConfig
	if err := json.Unmarshal(b, &s); err != nil {
		fatalf("failed to parse suites.json: %v", err)
	}
	return s
}

func mustExecuteSuite(suite, runID, label, cliArgs string, opts runOpts) runResult {
	// Prepare metadata
	sha := runCmdOutSilent("git", "rev-parse", "--short", "HEAD")
	branch := runCmdOutSilent("git", "rev-parse", "--abbrev-ref", "HEAD")
	goVer := runCmdOutSilent("go", "version")
	ffVer := strings.Split(runCmdOutSilent("ffmpeg", "-version"), "\n")[0]

	meta := runMetadata{
		RunID:        runID,
		Suite:        suite,
		Label:        label,
		CommitSHA:    strings.TrimSpace(sha),
		Branch:       strings.TrimSpace(branch),
		GoVersion:    strings.TrimSpace(goVer),
		Ffmpeg:       strings.TrimSpace(ffVer),
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		CLIArgs:      cliArgs,
		TimestampISO: time.Now().Format(time.RFC3339),
	}

	// End-to-end bench (count=1), collect ns/op etc.
	bench := runBench(cliArgs, opts)

	// Run CLI once to produce timings.json and count detections.
	timings, detections := runOnceForTimingsAndDetections(cliArgs, opts)

	return runResult{
		Metadata:   meta,
		Timings:    timings,
		Bench:      bench,
		Detections: detections,
	}
}

func runBench(cliArgs string, opts runOpts) benchStats {
	env := append(os.Environ(), "VLD_CLI_ARGS="+cliArgs)
	// Prefer a single iteration to reduce runtime noise; bench harness may still loop b.N internally.
	args := []string{"test", "-v", "-run", "^$", "-bench", "BenchmarkVideoLightningDetectorFromEnvArgs", "-benchmem", "-count", "1"}
	cmd := exec.Command("go", args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		fatalf("go test bench failed: %v\n%s", err, out)
	}
	s := string(out)
	if opts.echo {
		fmt.Print(s)
	}
	return parseBench(s)
}

var benchLine = regexp.MustCompile(`(?m)^BenchmarkVideoLightningDetectorFromEnvArgs\S*\s+\d+\s+([0-9\.e\+]+)\s+ns/op(?:\s+([0-9\.e\+]+)\s+B/op)?(?:\s+([0-9\.e\+]+)\s+allocs/op)?$`)

func parseBench(output string) benchStats {
	// Pick the last matching line.
	matches := benchLine.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return benchStats{}
	}
	m := matches[len(matches)-1]
	var ns, b, a float64
	fmt.Sscanf(m[1], "%f", &ns)
	if len(m) > 2 {
		fmt.Sscanf(m[2], "%f", &b)
	}
	if len(m) > 3 {
		fmt.Sscanf(m[3], "%f", &a)
	}
	return benchStats{NsPerOp: ns, BytesPerOp: b, AllocsPerOp: a}
}

func runOnceForTimingsAndDetections(cliArgs string, opts runOpts) (timingsReport, int) {
	// Ensure export-timings and verbose for counting detections; ensure -f to skip exports.
	args := parseArgs(cliArgs)
	if !hasArg(args, "--export-timings") {
		args = append(args, "--export-timings")
	}
	if !hasArg(args, "-v") && !hasArg(args, "--verbose") {
		args = append(args, "-v")
	}
	if !hasArg(args, "-f") && !hasArg(args, "--skip-frames-export") {
		args = append(args, "-f")
	}

	bin := filepath.Join(".", "bin", "video-lightning-detector")
	if opts.echo {
		fmt.Printf("detector> %s %s\n", bin, strings.Join(args, " "))
	}
	cmd := exec.Command(bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fatalf("failed to capture stdout: %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatalf("failed to start detector: %v", err)
	}
	var reader io.Reader = stdout
	if opts.stream {
		reader = io.TeeReader(stdout, os.Stdout)
	}
	detections := countDetections(reader)
	if err := cmd.Wait(); err != nil {
		fatalf("detector failed: %v", err)
	}

	// Find output directory to read timings.json
	outDir := findOutputDir(args)
	if outDir == "" {
		fatalf("could not determine output directory from CLI args")
	}
	tj := filepath.Join(outDir, "timings.json")
	f, err := os.Open(tj)
	if err != nil {
		fatalf("failed to open %s: %v", tj, err)
	}
	defer f.Close()
	var tr timingsReport
	if err := json.NewDecoder(f).Decode(&tr); err != nil {
		fatalf("failed to decode timings.json: %v", err)
	}
	return tr, detections
}

func countDetections(r io.Reader) int {
	s := bufio.NewScanner(r)
	count := 0
	for s.Scan() {
		line := s.Text()
		if strings.Contains(line, "Frame meets the threshold requirements.") {
			count++
		}
	}
	return count
}

func findOutputDir(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" || args[i] == "--output-directory-path" {
			if i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	return ""
}

func mustWriteRun(suite, runID string, res runResult) {
	dir := filepath.Join("perf-results", suite)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatalf("failed to create dir: %v", err)
	}
	p := filepath.Join(dir, runID+".json")
	f, err := os.Create(p)
	if err != nil {
		fatalf("failed to create run file: %v", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	if err := enc.Encode(res); err != nil {
		fatalf("failed to write run file: %v", err)
	}
}

func mustReadRun(suite, runID string) runResult {
	p := filepath.Join("perf-results", suite, runID+".json")
	b, err := os.ReadFile(p)
	if err != nil {
		fatalf("failed to read %s: %v", p, err)
	}
	var r runResult
	if err := json.Unmarshal(b, &r); err != nil {
		fatalf("failed to decode %s: %v", p, err)
	}
	return r
}

func loadBaselineID(suite string) (string, bool) {
	p := filepath.Join("perf-results", suite, "baseline.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	var m struct {
		RunID string `json:"run_id"`
	}
	if json.Unmarshal(b, &m) != nil || m.RunID == "" {
		return "", false
	}
	return m.RunID, true
}

func mustWriteBaselineID(suite, runID string) {
	dir := filepath.Join("perf-results", suite)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatalf("failed to create dir: %v", err)
	}
	p := filepath.Join(dir, "baseline.json")
	f, err := os.Create(p)
	if err != nil {
		fatalf("failed to create baseline file: %v", err)
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(map[string]string{"run_id": runID})
}

func printComparison(lhsID string, lhs runResult, rhsID string, rhs runResult, thresholdPct float64) {
	fmt.Printf("\nCompare %s -> %s (threshold %.1f%%)\n", lhsID, rhsID, thresholdPct)
	cmp := func(name string, a, b float64) {
		if a == 0 && b == 0 {
			fmt.Printf("- %-20s: n/a\n", name)
			return
		}
		delta := b - a
		pct := 0.0
		if a != 0 {
			pct = (delta / a) * 100.0
		}
		flag := ""
		if pct > thresholdPct {
			flag = "  REGRESSION"
		}
		fmt.Printf("- %-20s: %.0f -> %.0f  (%+.1f%%)%s\n", name, a, b, pct, flag)
	}
	cmp("total_ms", lhs.Timings.TotalMs, rhs.Timings.TotalMs)
	cmp("analysis_ms", lhs.Timings.Stages["video_analysis"], rhs.Timings.Stages["video_analysis"])
	cmp("detection_ms", lhs.Timings.Stages["video_detection"], rhs.Timings.Stages["video_detection"])
	cmp("ns/op", lhs.Bench.NsPerOp, rhs.Bench.NsPerOp)
	cmp("B/op", lhs.Bench.BytesPerOp, rhs.Bench.BytesPerOp)
	cmp("allocs/op", lhs.Bench.AllocsPerOp, rhs.Bench.AllocsPerOp)
}

func runCmdOutSilent(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

// parseArgs: simple shell-like splitter that respects quotes.
func parseArgs(s string) []string {
	args := []string{}
	buf := strings.Builder{}
	inSingle := false
	inDouble := false
	flush := func() {
		if buf.Len() > 0 {
			args = append(args, buf.String())
			buf.Reset()
		}
	}
	for _, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
				continue
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
				continue
			}
		case ' ', '\t', '\n':
			if !inSingle && !inDouble {
				flush()
				continue
			}
		}
		buf.WriteRune(r)
	}
	flush()
	return args
}

func hasArg(args []string, key string) bool {
	for _, a := range args {
		if a == key {
			return true
		}
	}
	return false
}

func sanitize(s string) string {
	if s == "" {
		return "run"
	}
	// allow alnum, dash, underscore only
	out := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
	out = strings.Trim(out, "-")
	if out == "" {
		return "run"
	}
	return out
}
