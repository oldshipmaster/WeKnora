package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/mathmastery"
	"github.com/Tencent/WeKnora/internal/types"
)

func main() {
	baseURL := flag.String("base-url", "http://localhost:18080", "WeKnora backend base URL")
	email := flag.String("email", "math-master@local.weknora", "local bootstrap account email")
	passwordEnv := flag.String("password-env", "WEKNORA_MATH_ADMIN_PASSWORD", "environment variable containing the bootstrap password")
	manifestPath := flag.String("manifest", "/Volumes/extfastdata01/WeKnora-runtime/math-mastery/manifest.json", "material manifest path")
	curriculumPath := flag.String("curriculum", "data/math-mastery/pep-primary-math.json", "curriculum JSON path")
	textDir := flag.String("text-dir", "", "optional directory containing <target_id>.md extracted textbooks")
	examTextDir := flag.String("exam-text-dir", "", "optional directory containing <target_id>.md OCR exam text")
	questionsPath := flag.String("questions", "/Volumes/extfastdata01/WeKnora-runtime/math-mastery/questions/full.json", "diagnostic question payload path; empty skips question import")
	flag.Parse()

	password := os.Getenv(*passwordEnv)
	if password == "" {
		fatal(fmt.Errorf("environment variable %s is required", *passwordEnv))
	}
	manifestData, err := os.ReadFile(*manifestPath)
	if err != nil {
		fatal(fmt.Errorf("read manifest: %w", err))
	}
	var manifest mathmastery.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		fatal(fmt.Errorf("decode manifest: %w", err))
	}
	curriculum, err := os.ReadFile(*curriculumPath)
	if err != nil {
		fatal(fmt.Errorf("read curriculum: %w", err))
	}
	textbooks := make(map[string]string)
	materials := make(map[string]string)
	if strings.TrimSpace(*textDir) != "" {
		for _, entry := range manifest.Entries {
			if entry.Kind != mathmastery.MaterialTextbook || entry.Status != mathmastery.StatusFound {
				continue
			}
			content, readErr := os.ReadFile(filepath.Join(*textDir, entry.TargetID+".md"))
			if readErr != nil {
				fatal(fmt.Errorf("read extracted textbook %s: %w", entry.TargetID, readErr))
			}
			textbooks[entry.TargetID] = string(content)
		}
	}
	if strings.TrimSpace(*examTextDir) != "" {
		for _, entry := range manifest.Entries {
			if entry.Kind != mathmastery.MaterialExam || entry.Status != mathmastery.StatusFound {
				continue
			}
			content, readErr := os.ReadFile(filepath.Join(*examTextDir, entry.TargetID+".md"))
			if readErr != nil {
				fatal(fmt.Errorf("read OCR exam %s: %w", entry.TargetID, readErr))
			}
			materials[entry.TargetID] = string(content)
		}
	}
	var questions []types.MathQuestion
	var questionLinks []types.MathQuestionNode
	if strings.TrimSpace(*questionsPath) != "" {
		questionData, readErr := os.ReadFile(*questionsPath)
		if readErr != nil {
			fatal(fmt.Errorf("read diagnostic questions: %w", readErr))
		}
		var questionSeed struct {
			Questions []types.MathQuestion     `json:"questions"`
			Links     []types.MathQuestionNode `json:"links"`
		}
		if decodeErr := json.Unmarshal(questionData, &questionSeed); decodeErr != nil {
			fatal(fmt.Errorf("decode diagnostic questions: %w", decodeErr))
		}
		if len(questionSeed.Questions) == 0 {
			fatal(fmt.Errorf("diagnostic question payload has no questions"))
		}
		questions = questionSeed.Questions
		questionLinks = questionSeed.Links
	}

	client := &http.Client{Timeout: 2 * time.Hour}
	report, err := mathmastery.NewBootstrapClient(*baseURL, client).Run(context.Background(), mathmastery.BootstrapConfig{
		Email:             strings.TrimSpace(*email),
		Password:          password,
		KnowledgeBaseName: "人教版小学数学体系化掌握",
		Curriculum:        curriculum,
		Manifest:          manifest,
		TextbookText:      textbooks,
		MaterialText:      materials,
		Questions:         questions,
		QuestionLinks:     questionLinks,
	})
	if err != nil {
		fatal(err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(encoded))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
