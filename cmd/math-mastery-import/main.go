package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Tencent/WeKnora/internal/mathmastery"
)

func main() {
	root := flag.String("root", "/Volumes/extdownload01/ChinaTextbook", "material source root")
	out := flag.String("out", "-", "manifest output path, or - for stdout")
	flag.Parse()

	manifest, err := mathmastery.ScanMaterialManifest(*root)
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

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
