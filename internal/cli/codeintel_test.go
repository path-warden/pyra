package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// (captureStdout is defined in update_flags_test.go and reused here.)

// runCLI dispatches the root command with the given args, resetting the flags
// the code-intelligence commands share so state does not leak between tests.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	resetCodeIntelFlags()
	rootCmd.SetArgs(args)
	var execErr error
	out := captureStdout(t, func() { execErr = rootCmd.Execute() })
	return out, execErr
}

func resetCodeIntelFlags() {
	// Reset the flags we toggle in tests to their declared defaults, so global
	// cobra flag state does not leak between test cases.
	_ = outlineCmd.Flags().Set("json", "false")
	_ = outlineCmd.Flags().Set("detail", "1")
	_ = outlineCmd.Flags().Set("kind", "")
	_ = symbolsCmd.Flags().Set("json", "false")
	_ = symbolsCmd.Flags().Set("name", "")
	_ = definitionCmd.Flags().Set("json", "false")
}

const cliGoFixture = "package sample\n\ntype T struct{}\n\nfunc Widget() int { return 1 }\n"

func TestCLI_OutlineJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.go"), []byte(cliGoFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	out, err := runCLI(t, "outline", "s.go", "--json", "--detail", "1")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output was not JSON: %v\n%s", err, out)
	}
	found := false
	for _, r := range rows {
		if r["name"] == "Widget" {
			found = true
			if _, ok := r["id"]; !ok {
				t.Error("detail 1 should include id")
			}
		}
	}
	if !found {
		t.Errorf("expected Widget in outline JSON: %s", out)
	}
}

func TestCLI_SymbolsJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.go"), []byte(cliGoFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	out, err := runCLI(t, "symbols", ".", "--name", "Widget", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Widget") || !strings.Contains(out, "#Widget@") {
		t.Errorf("expected Widget symbol-id in JSON: %s", out)
	}
}

func TestCLI_RootEscapeRejected(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "x.go"), []byte(cliGoFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	_, err := runCLI(t, "outline", filepath.Join(outside, "x.go"), "--json")
	if err == nil {
		t.Fatal("expected an error outlining a file outside the working root")
	}
	if !strings.Contains(err.Error(), "escapes working root") {
		t.Errorf("expected a confinement error, got: %v", err)
	}
}

// TestCLI_CheckExitCode verifies `pyra check` exits non-zero on syntax errors
// and zero on a clean file, using the standard subprocess-of-test pattern (the
// command calls os.Exit, which cannot be observed in-process).
func TestCLI_CheckExitCode(t *testing.T) {
	if os.Getenv("PYRA_CLI_CHECK_SUBPROC") == "1" {
		// Child: run the check command for real; it may call os.Exit. chdir to
		// the file's directory so the default "." root confines to it.
		file := os.Getenv("PYRA_CLI_CHECK_FILE")
		_ = os.Chdir(filepath.Dir(file))
		rootCmd.SetArgs([]string{"check", filepath.Base(file)})
		_ = rootCmd.Execute()
		return
	}

	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.go")
	clean := filepath.Join(dir, "clean.go")
	if err := os.WriteFile(broken, []byte("package x\nfunc f( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clean, []byte(cliGoFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(file string) int {
		cmd := exec.Command(os.Args[0], "-test.run=TestCLI_CheckExitCode")
		cmd.Env = append(os.Environ(), "PYRA_CLI_CHECK_SUBPROC=1", "PYRA_CLI_CHECK_FILE="+file)
		err := cmd.Run()
		if err == nil {
			return 0
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		t.Fatalf("subprocess error: %v", err)
		return -1
	}

	if code := run(broken); code == 0 {
		t.Error("check on a broken file should exit non-zero")
	}
	if code := run(clean); code != 0 {
		t.Errorf("check on a clean file should exit 0, got %d", code)
	}
}
