package store

import (
	"context"
	"log"
	"os"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := NewStore(context.Background(), testDatabaseURL())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func testDatabaseURL() string {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		log.Panic("TEST_DATABASE_URL is required")
	}

	return url
}
