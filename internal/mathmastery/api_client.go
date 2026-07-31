package mathmastery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

const defaultMathKnowledgeBaseName = "人教版小学数学体系化掌握"

type BootstrapConfig struct {
	Email             string
	Password          string
	KnowledgeBaseName string
	Curriculum        []byte
	Manifest          Manifest
	TextbookText      map[string]string
	MaterialText      map[string]string
}

type BootstrapReport struct {
	KnowledgeBaseID   string   `json:"knowledge_base_id"`
	UploadedTextbooks int      `json:"uploaded_textbooks"`
	SkippedTextbooks  int      `json:"skipped_textbooks"`
	UploadedExams     int      `json:"uploaded_exams"`
	SkippedExams      int      `json:"skipped_exams"`
	MissingExams      []string `json:"missing_exams"`
}

type BootstrapClient struct {
	baseURL string
	http    *http.Client
	token   string
}

func NewBootstrapClient(baseURL string, client *http.Client) *BootstrapClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &BootstrapClient{
		baseURL: strings.TrimRight(baseURL, "/") + "/api/v1",
		http:    client,
	}
}

func (c *BootstrapClient) Run(ctx context.Context, cfg BootstrapConfig) (BootstrapReport, error) {
	if strings.TrimSpace(cfg.Email) == "" || cfg.Password == "" {
		return BootstrapReport{}, errors.New("bootstrap email and password are required")
	}
	seed, err := BuildCurriculumSeed(cfg.Curriculum)
	if err != nil {
		return BootstrapReport{}, err
	}
	if err := c.authenticate(ctx, cfg.Email, cfg.Password); err != nil {
		return BootstrapReport{}, err
	}
	name := strings.TrimSpace(cfg.KnowledgeBaseName)
	if name == "" {
		name = defaultMathKnowledgeBaseName
	}
	kbID, err := c.findOrCreateKnowledgeBase(ctx, name)
	if err != nil {
		return BootstrapReport{}, err
	}

	report := BootstrapReport{KnowledgeBaseID: kbID, MissingExams: []string{}}
	uploaded := make(map[string]UploadedKnowledge)
	materialText := make(map[string]string, len(cfg.TextbookText)+len(cfg.MaterialText))
	for targetID, content := range cfg.TextbookText {
		materialText[targetID] = content
	}
	for targetID, content := range cfg.MaterialText {
		materialText[targetID] = content
	}
	existing := map[string]knowledgeRecord{}
	if len(materialText) > 0 {
		existing, err = c.listKnowledge(ctx, kbID)
		if err != nil {
			return report, fmt.Errorf("list existing material knowledge: %w", err)
		}
	}
	for _, entry := range cfg.Manifest.Entries {
		if entry.Kind == MaterialExam && entry.Status == StatusMissing {
			report.MissingExams = append(report.MissingExams, entry.Title)
			continue
		}
		if entry.Status != StatusFound {
			continue
		}
		if content := strings.TrimSpace(materialText[entry.TargetID]); content != "" {
			if knowledge, ok := existing[entry.Title]; ok {
				incrementSkippedMaterial(&report, entry.Kind)
				uploaded[entry.TargetID] = UploadedKnowledge{ID: knowledge.ID, Status: materialStatusFromParseStatus(knowledge.ParseStatus)}
				continue
			}
			knowledgeID, status, uploadErr := c.uploadManualMaterial(ctx, kbID, entry, content)
			if uploadErr != nil {
				return report, uploadErr
			}
			incrementUploadedMaterial(&report, entry.Kind)
			uploaded[entry.TargetID] = UploadedKnowledge{ID: knowledgeID, Status: status}
			continue
		}
		if entry.Kind != MaterialTextbook {
			continue
		}
		knowledgeID, status, duplicate, uploadErr := c.uploadTextbook(ctx, kbID, entry)
		if uploadErr != nil {
			return report, uploadErr
		}
		if duplicate {
			report.SkippedTextbooks++
		} else {
			report.UploadedTextbooks++
		}
		uploaded[entry.TargetID] = UploadedKnowledge{ID: knowledgeID, Status: status}
	}

	if err := c.sendJSON(ctx, http.MethodPost, "/knowledge-bases/"+url.PathEscape(kbID)+"/math-mastery/seed", seed, nil); err != nil {
		return report, fmt.Errorf("seed mastery curriculum: %w", err)
	}
	sources := struct {
		Sources []types.MathSourceBinding `json:"sources"`
	}{Sources: BuildSourceBindings(cfg.Manifest, uploaded)}
	if err := c.sendJSON(ctx, http.MethodPut, "/knowledge-bases/"+url.PathEscape(kbID)+"/math-mastery/sources", sources, nil); err != nil {
		return report, fmt.Errorf("write mastery sources: %w", err)
	}
	return report, nil
}

func incrementUploadedMaterial(report *BootstrapReport, kind MaterialKind) {
	if kind == MaterialExam {
		report.UploadedExams++
		return
	}
	report.UploadedTextbooks++
}

func incrementSkippedMaterial(report *BootstrapReport, kind MaterialKind) {
	if kind == MaterialExam {
		report.SkippedExams++
		return
	}
	report.SkippedTextbooks++
}

type knowledgeRecord struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ParseStatus string `json:"parse_status"`
}

func (c *BootstrapClient) listKnowledge(ctx context.Context, kbID string) (map[string]knowledgeRecord, error) {
	var response struct {
		Data []knowledgeRecord `json:"data"`
	}
	path := "/knowledge-bases/" + url.PathEscape(kbID) + "/knowledge?page=1&page_size=100"
	if err := c.sendJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	byTitle := make(map[string]knowledgeRecord, len(response.Data))
	for _, knowledge := range response.Data {
		byTitle[knowledge.Title] = knowledge
	}
	return byTitle, nil
}

func (c *BootstrapClient) authenticate(ctx context.Context, email, password string) error {
	if err := c.login(ctx, email, password); err == nil {
		return nil
	}
	register := map[string]string{"username": "math-master", "email": email, "password": password}
	if err := c.sendPublicJSON(ctx, http.MethodPost, "/auth/register", register, nil); err != nil {
		return fmt.Errorf("register bootstrap account: %w", err)
	}
	if err := c.login(ctx, email, password); err != nil {
		return fmt.Errorf("login bootstrap account: %w", err)
	}
	return nil
}

func (c *BootstrapClient) login(ctx context.Context, email, password string) error {
	var response struct {
		Token string `json:"token"`
	}
	if err := c.sendPublicJSON(ctx, http.MethodPost, "/auth/login", map[string]string{
		"email": email, "password": password,
	}, &response); err != nil {
		return err
	}
	if response.Token == "" {
		return errors.New("login response did not include a token")
	}
	c.token = response.Token
	return nil
}

func (c *BootstrapClient) findOrCreateKnowledgeBase(ctx context.Context, name string) (string, error) {
	var list struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.sendJSON(ctx, http.MethodGet, "/knowledge-bases", nil, &list); err != nil {
		return "", fmt.Errorf("list knowledge bases: %w", err)
	}
	for _, kb := range list.Data {
		if kb.Name == name {
			return kb.ID, nil
		}
	}

	payload := map[string]any{
		"name":        name,
		"description": "用于判断孩子是否体系化掌握小学数学；教材负责知识证据，指定 2026 试卷负责诊断证据。",
		"type":        "document",
		"chunking_config": map[string]any{
			"chunk_size": 1200, "chunk_overlap": 120, "strategy": "auto", "languages": []string{"zh"},
		},
		"embedding_model_id": "builtin-dashscope-text-embedding-v4",
		"summary_model_id":   "builtin-dashscope-qwen37-plus",
		"wiki_config": map[string]any{
			"synthesis_model_id": "builtin-dashscope-qwen37-plus", "extraction_granularity": "standard",
			"content_instructions": "按小学数学知识点组织，保留年级、学期、单元和前置依赖。",
		},
		"indexing_strategy": map[string]bool{
			"vector_enabled": true, "keyword_enabled": true, "wiki_enabled": true, "graph_enabled": true,
		},
		"extract_config": map[string]any{
			"enabled": true,
			"text":    "分数表示把一个整体平均分成若干份，其中的一份或几份。",
			"tags":    []string{"知识点", "数学概念", "方法", "前置知识"},
			"nodes": []map[string]any{
				{"name": "分数", "attributes": []string{"三年级", "数与代数"}},
				{"name": "平均分", "attributes": []string{"前置知识", "方法"}},
			},
			"relations":           []map[string]string{{"node1": "平均分", "node2": "分数", "type": "前置于"}},
			"custom_instructions": "只抽取小学数学概念、方法、性质和明确的前置关系；名称使用教材术语，避免把例题编号当作节点。",
		},
		"question_generation_config": map[string]any{"enabled": false, "question_count": 3},
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.sendJSON(ctx, http.MethodPost, "/knowledge-bases", payload, &created); err != nil {
		return "", fmt.Errorf("create knowledge base: %w", err)
	}
	if created.Data.ID == "" {
		return "", errors.New("create knowledge base response did not include an ID")
	}
	return created.Data.ID, nil
}

func (c *BootstrapClient) uploadTextbook(ctx context.Context, kbID string, entry ManifestEntry) (string, MaterialStatus, bool, error) {
	file, err := os.Open(entry.Path)
	if err != nil {
		return "", "", false, fmt.Errorf("open textbook %q: %w", entry.Path, err)
	}
	defer file.Close()

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	writeErr := make(chan error, 1)
	go func() {
		defer close(writeErr)
		part, partErr := multipartWriter.CreateFormFile("file", filepath.Base(entry.Path))
		if partErr == nil {
			_, partErr = io.Copy(part, file)
		}
		if partErr == nil {
			partErr = multipartWriter.WriteField("fileName", filepath.Base(entry.Path))
		}
		if partErr == nil {
			partErr = multipartWriter.WriteField("metadata", fmt.Sprintf(`{"target_id":%q,"content_hash":%q}`, entry.TargetID, entry.ContentHash))
		}
		if partErr == nil {
			partErr = multipartWriter.WriteField("process_config", `{"graph_enabled":true,"extract_config":{"enabled":true}}`)
		}
		closeErr := multipartWriter.Close()
		if partErr == nil {
			partErr = closeErr
		}
		_ = writer.CloseWithError(partErr)
		writeErr <- partErr
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/knowledge-bases/"+url.PathEscape(kbID)+"/knowledge/file"), reader)
	if err != nil {
		return "", "", false, err
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	c.authorize(req)
	response, err := c.http.Do(req)
	if err != nil {
		return "", "", false, fmt.Errorf("upload %q: %w", entry.Title, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return "", "", false, err
	}
	if streamErr := <-writeErr; streamErr != nil {
		return "", "", false, fmt.Errorf("stream %q: %w", entry.Title, streamErr)
	}
	duplicate := response.StatusCode == http.StatusConflict
	if !duplicate && (response.StatusCode < 200 || response.StatusCode >= 300) {
		return "", "", false, apiStatusError(response, body)
	}
	var decoded struct {
		Data struct {
			ID          string `json:"id"`
			ParseStatus string `json:"parse_status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", "", false, fmt.Errorf("decode upload response: %w", err)
	}
	if decoded.Data.ID == "" {
		return "", "", false, errors.New("upload response did not include a knowledge ID")
	}
	return decoded.Data.ID, materialStatusFromParseStatus(decoded.Data.ParseStatus), duplicate, nil
}

func (c *BootstrapClient) uploadManualTextbook(ctx context.Context, kbID string, entry ManifestEntry, content string) (string, MaterialStatus, error) {
	return c.uploadManualMaterial(ctx, kbID, entry, content)
}

func (c *BootstrapClient) uploadManualMaterial(ctx context.Context, kbID string, entry ManifestEntry, content string) (string, MaterialStatus, error) {
	graphEnabled := entry.Kind == MaterialTextbook
	payload := map[string]any{
		"title":   entry.Title,
		"content": content,
		"status":  "publish",
		"channel": "api",
		"process_config": map[string]any{
			"graph_enabled": graphEnabled,
			"extract_config": map[string]any{
				"enabled": graphEnabled,
			},
		},
	}
	var response struct {
		Data knowledgeRecord `json:"data"`
	}
	path := "/knowledge-bases/" + url.PathEscape(kbID) + "/knowledge/manual"
	if err := c.sendJSON(ctx, http.MethodPost, path, payload, &response); err != nil {
		return "", "", fmt.Errorf("upload extracted material %q: %w", entry.Title, err)
	}
	if response.Data.ID == "" {
		return "", "", errors.New("manual textbook response did not include a knowledge ID")
	}
	return response.Data.ID, materialStatusFromParseStatus(response.Data.ParseStatus), nil
}

func materialStatusFromParseStatus(status string) MaterialStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
		return StatusReady
	case "failed", "cancelled":
		return StatusFailed
	default:
		return StatusProcessing
	}
}

func (c *BootstrapClient) sendJSON(ctx context.Context, method, path string, payload, result any) error {
	return c.send(ctx, method, path, payload, result, true)
}

func (c *BootstrapClient) sendPublicJSON(ctx context.Context, method, path string, payload, result any) error {
	return c.send(ctx, method, path, payload, result, false)
}

func (c *BootstrapClient) send(ctx context.Context, method, path string, payload, result any, authenticated bool) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path), body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		c.authorize(req)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return apiStatusError(response, data)
	}
	if result != nil && len(data) > 0 {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("decode %s %s response: %w", method, path, err)
		}
	}
	return nil
}

func (c *BootstrapClient) endpoint(path string) string {
	return c.baseURL + "/" + strings.TrimLeft(path, "/")
}

func (c *BootstrapClient) authorize(request *http.Request) {
	request.Header.Set("Authorization", "Bearer "+c.token)
}

func apiStatusError(response *http.Response, body []byte) error {
	message := strings.TrimSpace(string(body))
	if len(message) > 600 {
		message = message[:600]
	}
	return fmt.Errorf("API returned %s: %s", response.Status, message)
}
