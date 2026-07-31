package mathmastery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MaterialKind string

const (
	MaterialTextbook MaterialKind = "textbook"
	MaterialExam     MaterialKind = "exam"
)

type MaterialStatus string

const (
	StatusFound      MaterialStatus = "found"
	StatusMissing    MaterialStatus = "missing"
	StatusUploaded   MaterialStatus = "uploaded"
	StatusProcessing MaterialStatus = "processing"
	StatusReady      MaterialStatus = "ready"
	StatusFailed     MaterialStatus = "failed"
)

type Manifest struct {
	Version int             `json:"version"`
	Root    string          `json:"root"`
	Entries []ManifestEntry `json:"entries"`
}

type ManifestEntry struct {
	TargetID    string         `json:"target_id"`
	MaterialID  string         `json:"material_id,omitempty"`
	Kind        MaterialKind   `json:"kind"`
	Status      MaterialStatus `json:"status"`
	Title       string         `json:"title"`
	Edition     string         `json:"edition"`
	Grade       int            `json:"grade"`
	Term        int            `json:"term"`
	SchoolYear  int            `json:"school_year,omitempty"`
	Season      string         `json:"season,omitempty"`
	Path        string         `json:"path,omitempty"`
	Size        int64          `json:"size,omitempty"`
	ModifiedAt  int64          `json:"modified_at,omitempty"`
	ContentHash string         `json:"content_hash,omitempty"`
}

type materialTarget struct {
	entry ManifestEntry
	match func(string) bool
}

func (m Manifest) EntryByTarget(targetID string) (ManifestEntry, bool) {
	for _, entry := range m.Entries {
		if entry.TargetID == targetID {
			return entry, true
		}
	}
	return ManifestEntry{}, false
}

func ScanMaterialManifest(root string) (Manifest, error) {
	return ScanMaterialManifestWithRoots(root)
}

// ScanMaterialManifestWithRoots scans the canonical textbook root plus optional
// sibling roots used for edition-specific exam material. Entry paths remain
// absolute so downstream import can consume material from any scanned root.
func ScanMaterialManifestWithRoots(root string, additionalRoots ...string) (Manifest, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("resolve material root: %w", err)
	}

	roots := []string{absRoot}
	for _, root := range additionalRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		resolved, resolveErr := filepath.Abs(root)
		if resolveErr != nil {
			return Manifest{}, fmt.Errorf("resolve additional material root %q: %w", root, resolveErr)
		}
		roots = append(roots, resolved)
	}

	fileSet := make(map[string]struct{}, 64)
	for _, scanRoot := range roots {
		walkErr := filepath.WalkDir(scanRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
				fileSet[path] = struct{}{}
			}
			return nil
		})
		if walkErr != nil {
			return Manifest{}, fmt.Errorf("scan material root %q: %w", scanRoot, walkErr)
		}
	}
	files := make([]string, 0, len(fileSet))
	for path := range fileSet {
		files = append(files, path)
	}
	sort.Strings(files)

	manifest := Manifest{Version: 1, Root: absRoot, Entries: make([]ManifestEntry, 0, 24)}
	for _, target := range materialTargets() {
		entry := target.entry
		entry.Status = StatusMissing
		for _, path := range files {
			if !target.match(normalizeMaterialName(path)) {
				continue
			}
			info, statErr := os.Stat(path)
			if statErr != nil {
				return Manifest{}, fmt.Errorf("stat material %q: %w", path, statErr)
			}
			digest, hashErr := hashFile(path)
			if hashErr != nil {
				return Manifest{}, fmt.Errorf("hash material %q: %w", path, hashErr)
			}
			entry.Status = StatusFound
			entry.Path = path
			entry.Size = info.Size()
			entry.ModifiedAt = info.ModTime().Unix()
			entry.ContentHash = digest
			entry.MaterialID = stableMaterialID(entry.TargetID, digest)
			break
		}
		manifest.Entries = append(manifest.Entries, entry)
	}

	return manifest, nil
}

func materialTargets() []materialTarget {
	grades := []string{"", "一年级", "二年级", "三年级", "四年级", "五年级", "六年级"}
	targets := make([]materialTarget, 0, 24)
	for grade := 1; grade <= 6; grade++ {
		for term := 1; term <= 2; term++ {
			termName := map[int]string{1: "上册", 2: "下册"}[term]
			gradeName := grades[grade]
			targets = append(targets, materialTarget{
				entry: ManifestEntry{
					TargetID: fmt.Sprintf("rj-g%d-s%d-textbook", grade, term),
					Kind:     MaterialTextbook,
					Title:    fmt.Sprintf("人教版小学数学%s%s", gradeName, termName),
					Edition:  "人教版",
					Grade:    grade,
					Term:     term,
				},
				match: func(path string) bool {
					return strings.Contains(path, "小学/数学/人教版/") &&
						strings.Contains(path, "义务教育教科书") &&
						strings.Contains(path, "数学") &&
						strings.Contains(path, gradeName) &&
						strings.Contains(path, termName)
				},
			})
		}

		gradeName := grades[grade]
		targets = append(targets,
			materialTarget{
				entry: ManifestEntry{
					TargetID:   fmt.Sprintf("rj-g%d-s2-xueba-2026-spring", grade),
					Kind:       MaterialExam,
					Title:      fmt.Sprintf("2026春人教版%s下册《学霸提优大试卷》数学", gradeName),
					Edition:    "人教版",
					Grade:      grade,
					Term:       2,
					SchoolYear: 2026,
					Season:     "spring",
				},
				match: examMatcher(grade, gradeName, 2, "学霸提优大试卷", []string{"26春", "2026春"}),
			},
			materialTarget{
				entry: ManifestEntry{
					TargetID:   fmt.Sprintf("rj-g%d-s1-wuxing-2026-autumn", grade),
					Kind:       MaterialExam,
					Title:      fmt.Sprintf("2026秋人教版%s上册《五星学霸》数学", gradeName),
					Edition:    "人教版",
					Grade:      grade,
					Term:       1,
					SchoolYear: 2026,
					Season:     "autumn",
				},
				match: examMatcher(grade, gradeName, 1, "五星学霸", []string{"26秋", "2026秋"}),
			},
		)
	}
	return targets
}

func examMatcher(grade int, gradeName string, term int, series string, yearTokens []string) func(string) bool {
	termName := map[int]string{1: "上册", 2: "下册"}[term]
	shortTerm := map[int]string{1: "上", 2: "下"}[term]
	shortVolume := fmt.Sprintf("%d%s", grade, shortTerm)
	return func(path string) bool {
		baseName := path
		if separator := strings.LastIndex(baseName, "/"); separator >= 0 {
			baseName = baseName[separator+1:]
		}
		if strings.Contains(baseName, "答案") || strings.Contains(baseName, "解析") || strings.Contains(baseName, "详解") {
			return false
		}
		hasVolume := strings.Contains(path, gradeName+termName) ||
			(strings.Contains(path, gradeName) && strings.Contains(path, termName)) ||
			strings.Contains(path, shortVolume)
		if !hasVolume || !strings.Contains(path, series) || !strings.Contains(path, "数学") ||
			!strings.Contains(path, "人教") {
			return false
		}
		for _, token := range yearTokens {
			if strings.Contains(path, token) {
				return true
			}
		}
		return false
	}
}

func normalizeMaterialName(value string) string {
	value = filepath.ToSlash(value)
	replacer := strings.NewReplacer(" ", "", "\t", "", "·", "", "（副本）", "")
	return replacer.Replace(value)
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func stableMaterialID(targetID, digest string) string {
	hash := sha256.Sum256([]byte(targetID + ":" + digest))
	return "material-" + hex.EncodeToString(hash[:8])
}
