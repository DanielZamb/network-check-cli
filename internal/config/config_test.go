package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAMLSubset(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "netcheck.yaml")
	content := `
targets:
  ping: ["9.9.9.9"]
  resolvers: ["1.1.1.1","8.8.8.8"]
bandwidth:
  iperf:
    enabled: true
    target: "10.0.0.2"
expected_plan:
  download_mbps: 500
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Targets.Ping[0]; got != "9.9.9.9" {
		t.Fatalf("unexpected ping target: %s", got)
	}
	if cfg.Bandwidth.Iperf.Target != "10.0.0.2" {
		t.Fatalf("unexpected iperf target")
	}
	if len(cfg.Targets.Resolvers) != 2 {
		t.Fatalf("expected resolvers, got %+v", cfg.Targets.Resolvers)
	}
}

func TestLoadRejectLocalhostIperf(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "netcheck.yaml")
	content := `
bandwidth:
  iperf:
    enabled: true
    target: "localhost:5201"
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadYAMLStripsInlineCommentsFromScalar(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "netcheck.yaml")
	content := `
bandwidth:
  iperf:
    enabled: true
    target: "192.168.40.29:5201" # <-- change to your Windows machine IPv4:port
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Bandwidth.Iperf.Target != "192.168.40.29:5201" {
		t.Fatalf("unexpected iperf target: %q", cfg.Bandwidth.Iperf.Target)
	}
}

func TestLoadYAMLParsesQuotedResolverList(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "netcheck.yaml")
	content := `
targets:
  resolvers: ["1.1.1.1", "8.8.8.8"]
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets.Resolvers) != 2 {
		t.Fatalf("expected 2 resolvers, got %+v", cfg.Targets.Resolvers)
	}
	if cfg.Targets.Resolvers[0] != "1.1.1.1" || cfg.Targets.Resolvers[1] != "8.8.8.8" {
		t.Fatalf("unexpected resolvers: %+v", cfg.Targets.Resolvers)
	}
}
