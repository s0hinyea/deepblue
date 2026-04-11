package db_test

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/services"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// safeBuffer is a mutex-protected bytes.Buffer safe for concurrent log writes.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestWatchReports_DetectsInsert starts the watcher, inserts a document into
// community_reports, and asserts the watcher logs the image URL within 8s.
func TestWatchReports_DetectsInsert(t *testing.T) {
	_ = godotenv.Load("../../.env")
	db.Connect()

	buf := &safeBuffer{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	watchCtx, watchCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer watchCancel()

	go services.WatchReports(watchCtx)

	// Allow the Atlas change stream handshake to complete.
	time.Sleep(2 * time.Second)

	imageURL := "https://deepblue-images.s3.us-east-1.amazonaws.com/test-watcher-trigger.jpg"

	insertCtx, insertCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer insertCancel()

	_, err := db.ReportsCollection.InsertOne(insertCtx, bson.M{
		"image_url":  imageURL,
		"_test":      true,
		"created_at": time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}

	t.Logf("Document inserted — waiting for watcher to fire...")

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), imageURL) {
			t.Logf("Watcher fired correctly.\nLog: %s", buf.String())
			return
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Errorf("Watcher did not log the image URL within timeout.\nLog output so far:\n%s", buf.String())
}
