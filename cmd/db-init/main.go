package main

import (
	"flag"
	"log"
	"os"

	"agile-manager/internal/shared"
)

func main() {
	databaseURL := flag.String("database", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	flag.Parse()

	if _, err := shared.NewStore(*databaseURL); err != nil {
		log.Fatalf("init database: %v", err)
	}
	log.Println("Database is ready")
}
