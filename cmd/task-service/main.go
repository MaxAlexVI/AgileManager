package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"agile-manager/internal/servicehost"
	"agile-manager/internal/shared"
)

func main() {
	addr := flag.String("addr", ":8082", "HTTP listen address")
	databaseURL := flag.String("database", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	flag.Parse()

	store, err := shared.NewStore(*databaseURL)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           servicehost.NewDomainServer(store, servicehost.DomainTasks).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Task Service is running at http://localhost%s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}
