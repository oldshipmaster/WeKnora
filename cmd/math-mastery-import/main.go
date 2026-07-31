package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tencent/WeKnora/internal/mathmastery"
)

func main() {
	root := flag.String("root", "/Volumes/extdownload01/ChinaTextbook", "material source root")
	var examRoots stringListFlag
	flag.Var(&examRoots, "exam-root", "additional exam source root (repeatable)")
	out := flag.String("out", "-", "manifest output path, or - for stdout")
	flag.Parse()

	manifest, err := mathmastery.ScanMaterialManifestWithRoots(*root, examRoots...)
	if err != nil {
		fatal(err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fatal(fmt.Errorf("encode manifest: %w", err))
	}
	data = append(data, '\n')

	if *out == "-" {
		if _, err := os.Stdout.Write(data); err != nil {
			fatal(err)
		}
		return
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(fmt.Errorf("create output directory: %w", err))
	}
	if err := os.WriteFile(*out, data, 0o600); err != nil {
		fatal(fmt.Errorf("write manifest: %w", err))
	}
}

type stringListFlag []string

func (values *stringListFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *stringListFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
