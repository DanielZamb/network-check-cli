package docs

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed man/*.md
var manFS embed.FS

var topics = map[string]string{
	"netcheck":    "man/netcheck.md",
	"run":         "man/run.md",
	"soak":        "man/soak.md",
	"compare":     "man/compare.md",
	"config":      "man/config.md",
	"exit-codes":  "man/exit-codes.md",
	"json-schema": "man/json-schema.md",
}

func Topics() []string {
	out := make([]string, 0, len(topics))
	for k := range topics {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func Get(topic string) (string, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		topic = "netcheck"
	}
	path, ok := topics[topic]
	if !ok {
		return "", fmt.Errorf("unknown man topic: %s", topic)
	}
	b, err := manFS.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Export(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for topic, path := range topics {
		b, err := manFS.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "netcheck-"+topic+".md"), b, 0o644); err != nil {
			return err
		}
		roff := markdownToRoff(topic, string(b))
		if err := os.WriteFile(filepath.Join(dir, "netcheck-"+topic+".1"), []byte(roff), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func markdownToRoff(topic, md string) string {
	return fmt.Sprintf(".TH NETCHECK-%s 1\n.SH NAME\nnetcheck-%s\n.SH DESCRIPTION\n%s\n", strings.ToUpper(topic), topic, strings.ReplaceAll(md, "\n", "\n.br\n"))
}
