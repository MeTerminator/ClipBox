package upload

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

type barrierReader struct {
	payload []byte
	started chan<- struct{}
	release <-chan struct{}
	once    bool
}

func (reader *barrierReader) Read(buffer []byte) (int, error) {
	if !reader.once {
		reader.once = true
		reader.started <- struct{}{}
		<-reader.release
	}
	if len(reader.payload) == 0 {
		return 0, io.EOF
	}
	n := copy(buffer, reader.payload)
	reader.payload = reader.payload[n:]
	return n, nil
}

func TestChunkUploadResumesAndCompletes(t *testing.T) {
	dataDir := t.TempDir()
	manager, err := NewManager(dataDir, 4, 1024, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("abcdefghij")
	hash := digest(payload)
	now := time.Now()

	status, err := manager.Init(hash, int64(len(payload)), now)
	if err != nil {
		t.Fatal(err)
	}
	if status.TotalChunks != 3 || len(status.UploadedChunks) != 0 {
		t.Fatalf("unexpected initial status: %#v", status)
	}
	if err := manager.WriteChunk(hash, 0, bytes.NewReader(payload[:4]), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := manager.WriteChunk(hash, 2, bytes.NewReader(payload[8:]), now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}

	resumed, err := manager.Init(hash, int64(len(payload)), now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed.UploadedChunks) != 2 || resumed.UploadedChunks[0] != 0 || resumed.UploadedChunks[1] != 2 {
		t.Fatalf("resume did not report saved chunks: %#v", resumed.UploadedChunks)
	}
	if err := manager.WriteChunk(hash, 1, bytes.NewReader(payload[4:8]), now.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}

	path, size, err := manager.Complete(hash, now.Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(payload)) {
		t.Fatalf("got size %d", size)
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, payload) {
		t.Fatalf("stored payload differs: %q", stored)
	}
	if _, err := os.Stat(manager.sessionDir(hash)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed temporary session still exists: %v", err)
	}
}

func TestExpiredUploadIsRemoved(t *testing.T) {
	manager, err := NewManager(t.TempDir(), 4, 1024, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("partial upload")
	hash := digest(payload)
	now := time.Now()
	if _, err := manager.Init(hash, int64(len(payload)), now); err != nil {
		t.Fatal(err)
	}
	removed, err := manager.CleanupExpired(now.Add(11 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed %d sessions, want 1", removed)
	}
	if _, err := manager.Status(hash, now.Add(11*time.Minute)); !errors.Is(err, ErrExpired) {
		t.Fatalf("status error = %v, want ErrExpired", err)
	}
}

func TestHashMismatchClearsChunksForRetry(t *testing.T) {
	manager, err := NewManager(t.TempDir(), 4, 1024, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("expected")
	hash := digest(payload)
	now := time.Now()
	status, err := manager.Init(hash, int64(len(payload)), now)
	if err != nil {
		t.Fatal(err)
	}
	wrong := []byte("xxxxxxxx")
	for index := 0; index < status.TotalChunks; index++ {
		start := index * int(status.ChunkSize)
		end := min(start+int(status.ChunkSize), len(wrong))
		if err := manager.WriteChunk(hash, index, bytes.NewReader(wrong[start:end]), now); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := manager.Complete(hash, now); !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("complete error = %v, want ErrHashMismatch", err)
	}
	resumed, err := manager.Status(hash, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed.UploadedChunks) != 0 {
		t.Fatalf("bad chunks were retained: %#v", resumed.UploadedChunks)
	}
}

func TestDifferentChunksAreWrittenConcurrently(t *testing.T) {
	manager, err := NewManager(t.TempDir(), 4, 1024, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("abcdefgh")
	hash := digest(payload)
	now := time.Now()
	if _, err := manager.Init(hash, int64(len(payload)), now); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	for index := 0; index < 2; index++ {
		chunk := append([]byte(nil), payload[index*4:(index+1)*4]...)
		go func(index int, chunk []byte) {
			results <- manager.WriteChunk(hash, index, &barrierReader{payload: chunk, started: started, release: release}, now)
		}(index, chunk)
	}

	concurrent := true
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			concurrent = false
		}
	}
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if !concurrent {
		t.Fatal("chunk writes were serialized")
	}
}

func digest(payload []byte) string {
	sum := sha1.Sum(payload)
	return hex.EncodeToString(sum[:])
}
