package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeDummyBin(t *testing.T, path string) {
	t.Helper()
	content := "#!/usr/bin/env bash\necho netcheck-test-bin\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write dummy bin: %v", err)
	}
}

func runInstall(t *testing.T, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"install.sh"}, args...)...)
	cmd.Env = env
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, string(out))
	}
}

func TestInstallCopiesBinaryAndUpdatesZshrc(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "custom-bin")
	src := filepath.Join(t.TempDir(), "netcheck-src")
	writeDummyBin(t, src)

	env := append(os.Environ(),
		"HOME="+home,
		"SHELL=/bin/zsh",
		"PATH=/usr/bin:/bin",
	)
	runInstall(t, env, "--bin-dir", binDir, "--source-bin", src)

	dest := filepath.Join(binDir, "netcheck")
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("installed binary is not executable: mode=%v", info.Mode())
	}

	zshrc := filepath.Join(home, ".zshrc")
	body, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("expected zshrc update: %v", err)
	}
	want := "export PATH=\"" + binDir + ":$PATH\""
	if !strings.Contains(string(body), want) {
		t.Fatalf("zshrc missing PATH export line: %q", want)
	}
}

func TestInstallSkipsPathUpdateWhenAlreadyInPath(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "already-on-path")
	src := filepath.Join(t.TempDir(), "netcheck-src")
	writeDummyBin(t, src)

	env := append(os.Environ(),
		"HOME="+home,
		"SHELL=/bin/zsh",
		"PATH="+binDir+":/usr/bin:/bin",
	)
	runInstall(t, env, "--bin-dir", binDir, "--source-bin", src)

	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("expected no zshrc update when bin dir already on PATH")
	}
}

func TestInstallNoPathUpdateFlag(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "manual-path")
	src := filepath.Join(t.TempDir(), "netcheck-src")
	writeDummyBin(t, src)

	env := append(os.Environ(),
		"HOME="+home,
		"SHELL=/bin/zsh",
		"PATH=/usr/bin:/bin",
	)
	runInstall(t, env, "--bin-dir", binDir, "--source-bin", src, "--no-path-update")

	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("expected no zshrc update with --no-path-update")
	}
}
