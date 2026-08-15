package server

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MeTerminator/ClipBox/internal/config"
	"github.com/MeTerminator/ClipBox/internal/model"
	"github.com/MeTerminator/ClipBox/internal/store"
	"github.com/MeTerminator/ClipBox/internal/upload"
	"github.com/gin-gonic/gin"
)

type memoryStore struct {
	clips map[string]*model.Clip
}

func (m *memoryStore) Create(_ context.Context, clip *model.Clip) error {
	if _, exists := m.clips[clip.Code]; exists {
		return store.ErrConflict
	}
	copy := *clip
	if copy.CreatedAt.IsZero() {
		copy.CreatedAt = time.Now().UTC()
		clip.CreatedAt = copy.CreatedAt
	}
	m.clips[clip.Code] = &copy
	return nil
}

func (m *memoryStore) ConsumeByCode(_ context.Context, code string, now time.Time) (*model.Clip, error) {
	clip := m.clips[code]
	if clip == nil || clip.Expired(now) || clip.AccessCount <= 0 {
		return nil, store.ErrNotFound
	}
	clip.AccessCount--
	copy := *clip
	return &copy, nil
}

func (m *memoryStore) FindByCode(_ context.Context, code string) (*model.Clip, error) {
	clip := m.clips[code]
	if clip == nil {
		return nil, store.ErrNotFound
	}
	copy := *clip
	return &copy, nil
}

func (m *memoryStore) FindText(_ context.Context, hash string, now time.Time) (*model.Clip, error) {
	for _, clip := range m.clips {
		if clip.ContentType == model.ContentText && clip.ContentHash == hash && !clip.Expired(now) {
			copy := *clip
			return &copy, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *memoryStore) FindFile(_ context.Context, hash, filename string, now time.Time) (*model.Clip, error) {
	for _, clip := range m.clips {
		if clip.ContentType == model.ContentFile && clip.File != nil && clip.File.SHA1 == hash && clip.File.Filename == filename && !clip.Expired(now) {
			copy := *clip
			return &copy, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *memoryStore) FindReusableFile(_ context.Context, hash string, size int64, now time.Time) (*model.Clip, error) {
	for _, clip := range m.clips {
		if clip.ContentType == model.ContentFile && clip.File != nil && clip.File.SHA1 == hash && clip.File.Size == size && !clip.Expired(now) {
			copy := *clip
			return &copy, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *memoryStore) CleanupExpired(_ context.Context, now time.Time) ([]string, int64, error) {
	var paths []string
	var expired int64
	for code, clip := range m.clips {
		if !clip.Expired(now) {
			continue
		}
		if clip.File != nil {
			paths = append(paths, clip.File.Path)
		}
		delete(m.clips, code)
		expired++
	}
	return paths, expired, nil
}
func (m *memoryStore) CountFileReferences(_ context.Context, path string, now time.Time) (int, error) {
	count := 0
	for _, clip := range m.clips {
		if clip.File != nil && clip.File.Path == path && !clip.Expired(now) {
			count++
		}
	}
	return count, nil
}
func (m *memoryStore) Close() error { return nil }

func TestPickupRedirectsToStableSHA1Routes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	payload := []byte("download body")
	sum := sha1.Sum(payload)
	fileHash := hex.EncodeToString(sum[:])
	filePath := filepath.Join(dataDir, fileHash)
	if err := os.WriteFile(filePath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	text := "hello，ClipBox"
	textHash := sha1String(text)
	database := &memoryStore{clips: map[string]*model.Clip{
		"12345": {
			Code: "12345", ContentType: model.ContentFile,
			File:          &model.File{Filename: "原始 文件.txt", Path: filePath, SHA1: fileHash, Size: int64(len(payload)), MIMEType: "text/plain"},
			AccessCount:   2,
			ExpireSeconds: 3600, CreatedAt: now,
		},
		"23456": {
			Code: "23456", ContentType: model.ContentText, Content: text, ContentHash: textHash,
			AccessCount: 2, ExpireSeconds: 3600, CreatedAt: now,
		},
		"34567": {
			Code: "34567", ContentType: model.ContentLink, Content: "https://example.com/original",
			AccessCount: 2, ExpireSeconds: 3600, CreatedAt: now,
		},
	}}
	uploadManager, err := upload.NewManager(dataDir, 4, 1024, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DataDir: dataDir, WWWRoot: filepath.Join(dataDir, "missing"), RealIPHeader: "X-Real-IP", MaxTextSize: 1024, MaxLinkLength: 2048, MaxUploadFileSize: 1024, UploadWorkers: 4}
	application := New(cfg, database, uploadManager)
	application.now = func() time.Time { return now }

	filePickup := performRequest(application.Handler(), http.MethodGet, "/clip/12345", nil)
	if filePickup.Code != http.StatusFound {
		t.Fatalf("file pickup status = %d, body = %s", filePickup.Code, filePickup.Body.String())
	}
	location := filePickup.Header().Get("Location")
	if !strings.HasPrefix(location, "/file/"+fileHash+"/") {
		t.Fatalf("file redirect = %q", location)
	}
	fileResponse := performRequest(application.Handler(), http.MethodGet, location, nil)
	if fileResponse.Code != http.StatusOK || fileResponse.Body.String() != string(payload) {
		t.Fatalf("file response status = %d, body = %q", fileResponse.Code, fileResponse.Body.String())
	}
	if disposition := fileResponse.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "attachment;") {
		t.Fatalf("content disposition = %q", disposition)
	}

	textPickup := performRequest(application.Handler(), http.MethodGet, "/clip/23456", nil)
	if textPickup.Code != http.StatusFound || textPickup.Header().Get("Location") != "/text/"+textHash {
		t.Fatalf("text redirect status = %d, location = %q", textPickup.Code, textPickup.Header().Get("Location"))
	}
	textResponse := performRequest(application.Handler(), http.MethodGet, textPickup.Header().Get("Location"), nil)
	if textResponse.Code != http.StatusOK || textResponse.Body.String() != text {
		t.Fatalf("text response status = %d, body = %q", textResponse.Code, textResponse.Body.String())
	}

	linkPickup := performRequest(application.Handler(), http.MethodGet, "/clip/34567", nil)
	if linkPickup.Code != http.StatusFound || linkPickup.Header().Get("Location") != "https://example.com/original" {
		t.Fatalf("link redirect status = %d, location = %q", linkPickup.Code, linkPickup.Header().Get("Location"))
	}
}

func TestClipResolveReturnsStructuredContentAndConsumesOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	fileHash := strings.Repeat("a", 40)
	database := &memoryStore{clips: map[string]*model.Clip{
		"12345": {
			Code: "12345", ContentType: model.ContentText, Content: "hello",
			AccessCount: 3, MaxCount: 3, ExpireSeconds: 3600, CreatedAt: now,
		},
		"23456": {
			Code: "23456", ContentType: model.ContentLink, Content: "https://example.com/path",
			AccessCount: 2, MaxCount: 2, ExpireSeconds: 7200, CreatedAt: now,
		},
		"34567": {
			Code: "34567", ContentType: model.ContentFile,
			File:        &model.File{Filename: "report.pdf", SHA1: fileHash, Size: 42},
			AccessCount: 1, MaxCount: 1, ExpireSeconds: 1800, CreatedAt: now,
		},
	}}
	dataDir := t.TempDir()
	uploadManager, err := upload.NewManager(dataDir, 4, 1024, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	application := New(config.Config{DataDir: dataDir, WWWRoot: filepath.Join(dataDir, "missing")}, database, uploadManager)
	application.now = func() time.Time { return now }

	for range 2 {
		info := performRequest(application.Handler(), http.MethodGet, "/clip/12345/info", nil)
		if info.Code != http.StatusOK {
			t.Fatalf("info status = %d, body = %s", info.Code, info.Body.String())
		}
		var metadata struct {
			RemainingCount int `json:"remaining_count"`
		}
		if err := json.Unmarshal(info.Body.Bytes(), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata.RemainingCount != 3 {
			t.Fatalf("metadata query consumed an access: remaining = %d", metadata.RemainingCount)
		}
	}

	tests := []struct {
		code           string
		contentType    string
		remaining      int
		content        string
		filename       string
		downloadURL    string
		expectedExpiry time.Time
	}{
		{"12345", model.ContentText, 2, "hello", "", "", now.Add(time.Hour)},
		{"23456", model.ContentLink, 1, "https://example.com/path", "", "", now.Add(2 * time.Hour)},
		{"34567", model.ContentFile, 0, "", "report.pdf", "/file/" + fileHash + "/report.pdf", now.Add(30 * time.Minute)},
	}
	for _, test := range tests {
		response := performRequest(application.Handler(), http.MethodPost, "/clip/"+test.code+"/resolve", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("info %s status = %d, body = %s", test.code, response.Code, response.Body.String())
		}
		var body struct {
			Type           string    `json:"type"`
			Content        string    `json:"content"`
			Filename       string    `json:"filename"`
			DownloadURL    string    `json:"download_url"`
			RemainingCount int       `json:"remaining_count"`
			ExpiresAt      time.Time `json:"expires_at"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Type != test.contentType || body.Content != test.content || body.Filename != test.filename ||
			body.DownloadURL != test.downloadURL || body.RemainingCount != test.remaining || !body.ExpiresAt.Equal(test.expectedExpiry) {
			t.Fatalf("unexpected info response for %s: %#v", test.code, body)
		}
	}

	exhaustedInfo := performRequest(application.Handler(), http.MethodGet, "/clip/34567/info", nil)
	if exhaustedInfo.Code != http.StatusOK || !strings.Contains(exhaustedInfo.Body.String(), `"remaining_count":0`) {
		t.Fatalf("exhausted info status = %d, body = %s", exhaustedInfo.Code, exhaustedInfo.Body.String())
	}
	exhausted := performRequest(application.Handler(), http.MethodPost, "/clip/34567/resolve", nil)
	if exhausted.Code != http.StatusNotFound {
		t.Fatalf("exhausted code status = %d", exhausted.Code)
	}
}

func TestDirectHashRoutesRejectUnknownContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	uploadManager, err := upload.NewManager(dataDir, 4, 1024, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	database := &memoryStore{clips: map[string]*model.Clip{}}
	cfg := config.Config{DataDir: dataDir, WWWRoot: filepath.Join(dataDir, "missing"), MaxUploadFileSize: 1024, UploadWorkers: 4}
	application := New(cfg, database, uploadManager)
	missingHash := strings.Repeat("a", 40)
	for _, path := range []string{"/text/" + missingHash, "/file/" + missingHash + "/missing.txt"} {
		response := performRequest(application.Handler(), http.MethodGet, path, nil)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d", path, response.Code)
		}
	}
}

func TestDefaultCreationLimitsAndInternalCleanup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	dataDir := t.TempDir()
	database := &memoryStore{clips: map[string]*model.Clip{}}
	uploadManager, err := upload.NewManager(dataDir, 4, 1024, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	application := New(config.Config{
		DataDir: dataDir, WWWRoot: filepath.Join(dataDir, "missing"),
		MaxTextSize: 1024, MaxLinkLength: 2048, MaxUploadFileSize: 1024, UploadWorkers: 1,
	}, database, uploadManager)
	application.now = func() time.Time { return now }

	request := httptest.NewRequest(http.MethodPost, "/clip/create", strings.NewReader("content=hello"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	application.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", response.Code, response.Body.String())
	}
	var created map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	clip := database.clips[created["code"]]
	if clip == nil || clip.AccessCount != 1000 || clip.MaxCount != 1000 || clip.ExpireSeconds != 86400 {
		t.Fatalf("default limits = %#v", clip)
	}

	expiredPath := filepath.Join(dataDir, "files", strings.Repeat("b", 40))
	if err := os.MkdirAll(filepath.Dir(expiredPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expiredPath, []byte("expired"), 0o600); err != nil {
		t.Fatal(err)
	}
	database.clips["99999"] = &model.Clip{
		Code: "99999", ContentType: model.ContentFile,
		File:        &model.File{Filename: "expired.txt", Path: expiredPath, SHA1: strings.Repeat("b", 40)},
		AccessCount: 1, MaxCount: 1, ExpireSeconds: 3600, CreatedAt: now.Add(-2 * time.Hour),
	}
	expired, removed, err := application.cleanupExpired(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if expired != 1 || removed != 1 {
		t.Fatalf("cleanup = expired %d, removed %d", expired, removed)
	}
	if _, err := os.Stat(expiredPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired file still exists: %v", err)
	}
	removedRoute := performRequest(application.Handler(), http.MethodGet, "/clip/timetask_cleanup_files", nil)
	if removedRoute.Code != http.StatusNotFound {
		t.Fatalf("removed cleanup route status = %d", removedRoute.Code)
	}
}

func TestChunkUploadHTTPFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	database := &memoryStore{clips: map[string]*model.Clip{}}
	uploadManager, err := upload.NewManager(dataDir, 4, 1024, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		DataDir: dataDir, WWWRoot: filepath.Join(dataDir, "missing"), RealIPHeader: "X-Real-IP",
		MaxTextSize: 1024, MaxLinkLength: 2048, MaxUploadFileSize: 1024, UploadWorkers: 2,
	}
	application := New(cfg, database, uploadManager)
	payload := []byte("abcdefghij")
	hashSum := sha1.Sum(payload)
	hash := hex.EncodeToString(hashSum[:])

	initResponse := performJSONRequest(t, application.Handler(), http.MethodPost, "/clip/upload/init", map[string]any{
		"filename": "sample.txt", "size": len(payload), "sha1": hash, "count": 2, "expire": 3600,
	})
	if initResponse.Code != http.StatusOK {
		t.Fatalf("init status = %d, body = %s", initResponse.Code, initResponse.Body.String())
	}
	var initialized struct {
		UploadID    string `json:"upload_id"`
		ChunkSize   int    `json:"chunk_size"`
		TotalChunks int    `json:"total_chunks"`
	}
	if err := json.Unmarshal(initResponse.Body.Bytes(), &initialized); err != nil {
		t.Fatal(err)
	}
	if initialized.UploadID != hash || initialized.ChunkSize != 4 || initialized.TotalChunks != 3 {
		t.Fatalf("unexpected init response: %#v", initialized)
	}

	for index := 0; index < initialized.TotalChunks; index++ {
		start := index * initialized.ChunkSize
		end := min(start+initialized.ChunkSize, len(payload))
		request := httptest.NewRequest(http.MethodPut, "/clip/upload/"+hash+"/"+strconv.Itoa(index), bytes.NewReader(payload[start:end]))
		request.Header.Set("Content-Type", "application/octet-stream")
		response := httptest.NewRecorder()
		application.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("chunk %d status = %d, body = %s", index, response.Code, response.Body.String())
		}
	}

	completeResponse := performJSONRequest(t, application.Handler(), http.MethodPost, "/clip/upload/"+hash+"/complete", map[string]any{
		"filename": "sample.txt", "count": 2, "expire": 3600,
	})
	if completeResponse.Code != http.StatusOK {
		t.Fatalf("complete status = %d, body = %s", completeResponse.Code, completeResponse.Body.String())
	}
	var completed struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal(completeResponse.Body.Bytes(), &completed); err != nil {
		t.Fatal(err)
	}
	if len(completed.Code) != 5 || completed.URL != "/file/"+hash+"/sample.txt" {
		t.Fatalf("unexpected complete response: %#v", completed)
	}
	pickup := performRequest(application.Handler(), http.MethodGet, "/clip/"+completed.Code, nil)
	if pickup.Code != http.StatusFound || pickup.Header().Get("Location") != completed.URL {
		t.Fatalf("pickup status = %d, location = %q", pickup.Code, pickup.Header().Get("Location"))
	}
	download := performRequest(application.Handler(), http.MethodGet, completed.URL, nil)
	if download.Code != http.StatusOK || !bytes.Equal(download.Body.Bytes(), payload) {
		t.Fatalf("download status = %d, body = %q", download.Code, download.Body.Bytes())
	}
}

func performRequest(handler http.Handler, method, target string, body io.Reader) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, body)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func performJSONRequest(t *testing.T, handler http.Handler, method, target string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

var _ store.Store = (*memoryStore)(nil)
