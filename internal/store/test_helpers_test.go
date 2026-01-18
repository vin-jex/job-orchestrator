package store

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func testDatabaseURL() string {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		log.Panic("TEST_DATABASE_URL is required")
	}

	return url
}
