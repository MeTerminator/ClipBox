package upload

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidUpload = errors.New("invalid upload")
	ErrExpired       = errors.New("upload session expired")
	ErrIncomplete    = errors.New("upload is incomplete")
	ErrHashMismatch  = errors.New("file SHA1 mismatch")
)

type Manifest struct {
	Version     int       `json:"version"`
	ID          string    `json:"id"`
	SHA1        string    `json:"sha1"`
	Size        int64     `json:"size"`
	ChunkSize   int64     `json:"chunk_size"`
	TotalChunks int       `json:"total_chunks"`
	CreatedAt   time.Time `json:"created_at"`
}

type Status struct {
	Manifest
	UploadedChunks []int     `json:"uploaded_chunks"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type Manager struct {
	tmpRoot   string
	filesRoot string
	chunkSize int64
	maxSize   int64
	ttl       time.Duration

	locksMu sync.Mutex
	locks   map[string]*sync.RWMutex
}

func NewManager(dataDir string, chunkSize, maxSize int64, ttl time.Duration) (*Manager, error) {
	manager := &Manager{
		tmpRoot:   filepath.Join(dataDir, "tmp"),
		filesRoot: filepath.Join(dataDir, "files"),
		chunkSize: chunkSize,
		maxSize:   maxSize,
		ttl:       ttl,
		locks:     make(map[string]*sync.RWMutex),
	}
	if err := os.MkdirAll(manager.tmpRoot, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(manager.filesRoot, 0o755); err != nil {
		return nil, err
	}
	return manager, nil
}

func ValidSHA1(value string) bool {
	if len(value) != sha1.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func (m *Manager) Init(hash string, size int64, now time.Time) (*Status, error) {
	if !ValidSHA1(hash) || size < 0 || size > m.maxSize {
		return nil, ErrInvalidUpload
	}
	unlock := m.exclusiveLock(hash)
	defer unlock()

	dir := m.sessionDir(hash)
	manifest, err := m.readManifest(hash)
	if err == nil {
		activity, activityErr := m.activityTime(hash)
		if activityErr != nil || now.Sub(activity) > m.ttl || manifest.Size != size || manifest.ChunkSize != m.chunkSize {
			if removeErr := os.RemoveAll(dir); removeErr != nil {
				return nil, removeErr
			}
			manifest = nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		if removeErr := os.RemoveAll(dir); removeErr != nil {
			return nil, removeErr
		}
	}

	if manifest == nil {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		totalChunks := 0
		if size > 0 {
			totalChunks = int((size + m.chunkSize - 1) / m.chunkSize)
		}
		manifest = &Manifest{
			Version:     1,
			ID:          hash,
			SHA1:        hash,
			Size:        size,
			ChunkSize:   m.chunkSize,
			TotalChunks: totalChunks,
			CreatedAt:   now.UTC(),
		}
		payload, marshalErr := json.Marshal(manifest)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if err := writeAtomic(filepath.Join(dir, "manifest.json"), payload, 0o600); err != nil {
			return nil, err
		}
	}
	if err := m.touch(hash, now); err != nil {
		return nil, err
	}
	return m.statusLocked(manifest, now)
}

func (m *Manager) Status(hash string, now time.Time) (*Status, error) {
	if !ValidSHA1(hash) {
		return nil, ErrInvalidUpload
	}
	unlock := m.exclusiveLock(hash)
	defer unlock()
	manifest, err := m.activeManifest(hash, now)
	if err != nil {
		return nil, err
	}
	if err := m.touch(hash, now); err != nil {
		return nil, err
	}
	return m.statusLocked(manifest, now)
}

func (m *Manager) WriteChunk(hash string, index int, input io.Reader, now time.Time) error {
	if !ValidSHA1(hash) {
		return ErrInvalidUpload
	}
	unlock := m.sharedLock(hash)
	defer unlock()
	manifest, err := m.activeManifest(hash, now)
	if err != nil {
		return err
	}
	if index < 0 || index >= manifest.TotalChunks {
		return ErrInvalidUpload
	}
	expected := manifest.ChunkSize
	if index == manifest.TotalChunks-1 {
		expected = manifest.Size - int64(index)*manifest.ChunkSize
	}

	temporary, err := os.CreateTemp(m.sessionDir(hash), fmt.Sprintf(".%08d-*.part", index))
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()

	written, copyErr := io.Copy(temporary, io.LimitReader(input, expected+1))
	if copyErr == nil && written != expected {
		copyErr = fmt.Errorf("chunk has %d bytes, expected %d: %w", written, expected, ErrInvalidUpload)
	}
	if syncErr := temporary.Sync(); copyErr == nil && syncErr != nil {
		copyErr = syncErr
	}
	if closeErr := temporary.Close(); copyErr == nil && closeErr != nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return copyErr
	}
	if err := os.Rename(temporaryName, m.chunkPath(hash, index)); err != nil {
		return err
	}
	return m.touch(hash, now)
}

func (m *Manager) Complete(hash string, now time.Time) (string, int64, error) {
	if !ValidSHA1(hash) {
		return "", 0, ErrInvalidUpload
	}
	unlock := m.exclusiveLock(hash)
	defer unlock()
	manifest, err := m.activeManifest(hash, now)
	if err != nil {
		return "", 0, err
	}
	status, err := m.statusLocked(manifest, now)
	if err != nil {
		return "", 0, err
	}
	if len(status.UploadedChunks) != manifest.TotalChunks {
		return "", 0, ErrIncomplete
	}

	assembledPath := filepath.Join(m.sessionDir(hash), "assembled.part")
	assembled, err := os.OpenFile(assembledPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, err
	}
	hasher := sha1.New()
	writer := io.MultiWriter(assembled, hasher)
	var copied int64
	for index := 0; index < manifest.TotalChunks; index++ {
		chunk, openErr := os.Open(m.chunkPath(hash, index))
		if openErr != nil {
			_ = assembled.Close()
			return "", 0, openErr
		}
		n, copyErr := io.Copy(writer, chunk)
		closeErr := chunk.Close()
		copied += n
		if copyErr != nil || closeErr != nil {
			_ = assembled.Close()
			if copyErr != nil {
				return "", 0, copyErr
			}
			return "", 0, closeErr
		}
	}
	if err := assembled.Sync(); err != nil {
		_ = assembled.Close()
		return "", 0, err
	}
	if err := assembled.Close(); err != nil {
		return "", 0, err
	}
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if copied != manifest.Size || actualHash != hash {
		_ = os.Remove(assembledPath)
		m.clearChunks(manifest)
		return "", 0, ErrHashMismatch
	}

	finalPath := m.FilePath(hash)
	if existing, statErr := os.Stat(finalPath); statErr == nil {
		if existing.Size() != copied {
			return "", 0, fmt.Errorf("stored SHA1 path has unexpected size")
		}
		existingHash, hashErr := hashFile(finalPath)
		if hashErr != nil || existingHash != hash {
			return "", 0, fmt.Errorf("stored SHA1 path failed verification")
		}
		_ = os.Remove(assembledPath)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", 0, statErr
	} else if err := os.Rename(assembledPath, finalPath); err != nil {
		return "", 0, err
	}
	if err := os.RemoveAll(m.sessionDir(hash)); err != nil {
		return "", 0, err
	}
	return finalPath, copied, nil
}

func (m *Manager) FilePath(hash string) string {
	return filepath.Join(m.filesRoot, hash)
}

func (m *Manager) CleanupExpired(now time.Time) (int, error) {
	entries, err := os.ReadDir(m.tmpRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() || !ValidSHA1(entry.Name()) {
			continue
		}
		hash := entry.Name()
		unlock := m.exclusiveLock(hash)
		activity, activityErr := m.activityTime(hash)
		if activityErr != nil || now.Sub(activity) > m.ttl {
			if removeErr := os.RemoveAll(m.sessionDir(hash)); removeErr != nil {
				unlock()
				return removed, removeErr
			}
			removed++
		}
		unlock()
	}
	return removed, nil
}

func (m *Manager) RunCleanup(ctx context.Context) {
	interval := m.ttl / 2
	if interval > time.Minute {
		interval = time.Minute
	}
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_, _ = m.CleanupExpired(now)
		}
	}
}

func (m *Manager) activeManifest(hash string, now time.Time) (*Manifest, error) {
	manifest, err := m.readManifest(hash)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrExpired
		}
		return nil, err
	}
	activity, err := m.activityTime(hash)
	if err != nil || now.Sub(activity) > m.ttl {
		_ = os.RemoveAll(m.sessionDir(hash))
		return nil, ErrExpired
	}
	return manifest, nil
}

func (m *Manager) readManifest(hash string) (*Manifest, error) {
	payload, err := os.ReadFile(filepath.Join(m.sessionDir(hash), "manifest.json"))
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, err
	}
	if manifest.ID != hash || manifest.SHA1 != hash || manifest.ChunkSize <= 0 || manifest.Size < 0 {
		return nil, ErrInvalidUpload
	}
	return &manifest, nil
}

func (m *Manager) statusLocked(manifest *Manifest, now time.Time) (*Status, error) {
	uploaded := make([]int, 0, manifest.TotalChunks)
	for index := 0; index < manifest.TotalChunks; index++ {
		info, err := os.Stat(m.chunkPath(manifest.ID, index))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		expected := manifest.ChunkSize
		if index == manifest.TotalChunks-1 {
			expected = manifest.Size - int64(index)*manifest.ChunkSize
		}
		if info.Size() == expected {
			uploaded = append(uploaded, index)
		} else {
			_ = os.Remove(m.chunkPath(manifest.ID, index))
		}
	}
	sort.Ints(uploaded)
	return &Status{Manifest: *manifest, UploadedChunks: uploaded, ExpiresAt: now.Add(m.ttl).UTC()}, nil
}

func (m *Manager) clearChunks(manifest *Manifest) {
	for index := 0; index < manifest.TotalChunks; index++ {
		_ = os.Remove(m.chunkPath(manifest.ID, index))
	}
}

func (m *Manager) uploadLock(hash string) *sync.RWMutex {
	m.locksMu.Lock()
	mutex := m.locks[hash]
	if mutex == nil {
		mutex = &sync.RWMutex{}
		m.locks[hash] = mutex
	}
	m.locksMu.Unlock()
	return mutex
}

func (m *Manager) exclusiveLock(hash string) func() {
	mutex := m.uploadLock(hash)
	mutex.Lock()
	return mutex.Unlock
}

func (m *Manager) sharedLock(hash string) func() {
	mutex := m.uploadLock(hash)
	mutex.RLock()
	return mutex.RUnlock
}

func (m *Manager) sessionDir(hash string) string { return filepath.Join(m.tmpRoot, hash) }

func (m *Manager) chunkPath(hash string, index int) string {
	return filepath.Join(m.sessionDir(hash), fmt.Sprintf("%08d.chunk", index))
}

func (m *Manager) activityTime(hash string) (time.Time, error) {
	info, err := os.Stat(filepath.Join(m.sessionDir(hash), ".activity"))
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func (m *Manager) touch(hash string, now time.Time) error {
	path := filepath.Join(m.sessionDir(hash), ".activity")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Chtimes(path, now, now)
}

func writeAtomic(path string, payload []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".manifest-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func ParseChunkIndex(raw string) (int, error) {
	index, err := strconv.Atoi(raw)
	if err != nil || index < 0 {
		return 0, ErrInvalidUpload
	}
	return index, nil
}
