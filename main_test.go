package main

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"
    "time"
)

// parseArgs splits a command-line string into arguments, respecting simple quotes.
// Supports double and single quotes and basic escaping of spaces via quotes.
func parseArgs(s string) []string {
    args := []string{}
    buf := make([]rune, 0, len(s))
    inSingle := false
    inDouble := false

    flush := func() {
        if len(buf) > 0 {
            args = append(args, string(buf))
            buf = buf[:0]
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
        buf = append(buf, r)
    }
    flush()
    return args
}

// replaceOrAppendFlag ensures a flag (short or long) is set to a specific value.
// If present, replaces its value; otherwise appends the flag and value.
func replaceOrAppendFlag(args []string, keys []string, value string) []string {
    if len(args) == 0 {
        return append(args, keys[0], value)
    }
    out := make([]string, 0, len(args)+2)
    replaced := false
    i := 0
    for i < len(args) {
        a := args[i]
        matched := false
        for _, k := range keys {
            if a == k {
                matched = true
                break
            }
        }
        if matched {
            // skip this flag and its value (if exists)
            i++
            if i < len(args) {
                i++
            }
            out = append(out, keys[0], value)
            replaced = true
            continue
        }
        out = append(out, a)
        i++
    }
    if !replaced {
        out = append(out, keys[0], value)
    }
    return out
}

// ensureFlag adds a boolean flag if not present.
func ensureFlag(args []string, key string) []string {
    for _, a := range args {
        if a == key {
            return args
        }
    }
    return append(args, key)
}

// runCLI invokes the CLI via the main package entrypoint by executing the built binary if present,
// otherwise it runs the Go program with `go run`-like semantics by calling main.main through a small shim.
// For the benchmark we rely on the binary if available for closer-to-real profiling; otherwise fall back.
func runCLI(b *testing.B, args []string) {
    // We invoke the Cobra root command by calling the binary if it exists.
    // Prefer the built binary path documented in the repo: ./bin/video-lightning-detector
    bin := filepath.Join(".", "bin", "video-lightning-detector")
    if _, err := os.Stat(bin); err == nil {
        // Execute the binary in a subprocess for each iteration.
        // Subprocess creation overhead is acceptable for end-to-end bench intent.
        b.ResetTimer()
        b.ReportAllocs()
        for i := 0; i < b.N; i++ {
            // Unique output dir per iteration to avoid cross-run interference.
            out := filepath.Join("runs", "bench", time.Now().Format("20060102-150405"), b.Name(), filepath.Base(os.TempDir()), "iter-"+time.Now().Format("150405.000000000"))
            argv := replaceOrAppendFlag(args, []string{"-o", "--output-directory-path"}, out)
            argv = ensureFlag(argv, "-f")

            cmd := exec.Command(bin, argv...)
            cmd.Stdout = os.Stdout
            cmd.Stderr = os.Stderr
            if err := cmd.Run(); err != nil {
                b.Fatalf("bench run failed: %v", err)
            }
        }
        return
    }

    // Fallback: call main() directly via a thin layer by setting os.Args.
    // This is less representative (no subprocess), but allows benches without a prebuilt binary.
    origArgs := os.Args
    defer func() { os.Args = origArgs }()
    b.ResetTimer()
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        out := filepath.Join("runs", "bench", time.Now().Format("20060102-150405"), b.Name(), "iter-"+time.Now().Format("150405.000000000"))
        argv := []string{"video-lightning-detector"}
        argv = append(argv, replaceOrAppendFlag(args, []string{"-o", "--output-directory-path"}, out)...)
        argv = ensureFlag(argv, "-f")
        os.Args = argv
        main()
    }
}

// BenchmarkVideoLightningDetectorFromEnvArgs runs end-to-end benches using VLD_CLI_ARGS.
// Example:
//   export VLD_CLI_ARGS='-i resources/samples/sample_yes.mp4 -o ./runs/bench -a -s 0.4 -f'
//   go test -v -run ^$ -bench BenchmarkVideoLightningDetectorFromEnvArgs -benchmem -count 5
func BenchmarkVideoLightningDetectorFromEnvArgs(b *testing.B) {
    raw := os.Getenv("VLD_CLI_ARGS")
    if raw == "" {
        raw = "-i resources/samples/sample_yes.mp4 -o ./runs/bench -a -s 0.4 -f"
    }
    args := parseArgs(raw)
    runCLI(b, args)
}
