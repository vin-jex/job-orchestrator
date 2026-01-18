package store

import (
	"log"
	"os"
)

func testDatabaseURL() string {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		log.Panic("TEST_DATABASE_URL is required")
	}

	return url
}
