package mathmastery

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestScanMaterialManifestFindsAllTwelveTextbooks(t *testing.T) {
	root := t.TempDir()
	bookDir := filepath.Join(root, "小学", "数学", "人教版")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for grade := 1; grade <= 6; grade++ {
		for _, term := range []string{"上册", "下册"} {
			name := fmt.Sprintf("义务教育教科书·数学%s%s.pdf", chineseGrade(grade), term)
			if err := os.WriteFile(filepath.Join(bookDir, name), []byte(fmt.Sprintf("g%d-%s", grade, term)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	manifest, err := ScanMaterialManifest(root)
	if err != nil {
		t.Fatalf("ScanMaterialManifest() error = %v", err)
	}

	if got := countEntries(manifest.Entries, MaterialTextbook, StatusFound); got != 12 {
		t.Fatalf("found textbook count = %d, want 12", got)
	}
	if got := countEntries(manifest.Entries, MaterialExam, StatusMissing); got != 12 {
		t.Fatalf("missing 2026 exam count = %d, want 12", got)
	}
	for _, entry := range manifest.Entries {
		if entry.Kind == MaterialTextbook && entry.Status == StatusFound && entry.ContentHash == "" {
			t.Fatalf("textbook %s has empty content hash", entry.TargetID)
		}
	}
}

func TestScanMaterialManifestDoesNotSubstituteOlderExamEditions(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, "小学", "教辅", "2025")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(legacyDir, "25春一年级下册人教版学霸提优大试卷数学.pdf")
	if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := ScanMaterialManifest(root)
	if err != nil {
		t.Fatalf("ScanMaterialManifest() error = %v", err)
	}

	entry, ok := manifest.EntryByTarget("rj-g1-s2-xueba-2026-spring")
	if !ok {
		t.Fatal("2026 spring target is absent from manifest")
	}
	if entry.Status != StatusMissing {
		t.Fatalf("2026 spring target status = %q, want %q", entry.Status, StatusMissing)
	}
}

func TestScanMaterialManifestProducesStableIdentityAndDeduplicatesCandidates(t *testing.T) {
	root := t.TempDir()
	bookDir := filepath.Join(root, "小学", "数学", "人教版")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	book := filepath.Join(bookDir, "义务教育教科书 · 数学一年级上册.pdf")
	if err := os.WriteFile(book, []byte("same-book"), 0o644); err != nil {
		t.Fatal(err)
	}
	duplicate := filepath.Join(bookDir, "义务教育教科书·数学一年级上册（副本）.pdf")
	if err := os.WriteFile(duplicate, []byte("same-book"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := ScanMaterialManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ScanMaterialManifest(root)
	if err != nil {
		t.Fatal(err)
	}

	firstEntry, _ := first.EntryByTarget("rj-g1-s1-textbook")
	secondEntry, _ := second.EntryByTarget("rj-g1-s1-textbook")
	if firstEntry.ContentHash == "" || firstEntry.ContentHash != secondEntry.ContentHash {
		t.Fatalf("content hash is not stable: %q vs %q", firstEntry.ContentHash, secondEntry.ContentHash)
	}
	if firstEntry.MaterialID == "" || firstEntry.MaterialID != secondEntry.MaterialID {
		t.Fatalf("material ID is not stable: %q vs %q", firstEntry.MaterialID, secondEntry.MaterialID)
	}
	if got := countTarget(first.Entries, "rj-g1-s1-textbook"); got != 1 {
		t.Fatalf("target entry count = %d, want 1", got)
	}
}

func countEntries(entries []ManifestEntry, kind MaterialKind, status MaterialStatus) int {
	count := 0
	for _, entry := range entries {
		if entry.Kind == kind && entry.Status == status {
			count++
		}
	}
	return count
}

func countTarget(entries []ManifestEntry, targetID string) int {
	count := 0
	for _, entry := range entries {
		if entry.TargetID == targetID {
			count++
		}
	}
	return count
}

func chineseGrade(grade int) string {
	return []string{"", "一年级", "二年级", "三年级", "四年级", "五年级", "六年级"}[grade]
}
