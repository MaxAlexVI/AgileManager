package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"agile-manager/internal/gateway"
	"agile-manager/internal/shared"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	databaseURL := flag.String("database", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	staticDir := flag.String("static", "web/static", "static web directory")
	flag.Parse()

	store, err := shared.NewStore(*databaseURL)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}

	server := &http.Server{
		Addr: *addr,
		Handler: gateway.NewServerWithRemotes(store, *staticDir, gateway.ServiceURLs{
			Users:         os.Getenv("USER_SERVICE_URL"),
			Tasks:         os.Getenv("TASK_SERVICE_URL"),
			Sprints:       os.Getenv("SPRINT_SERVICE_URL"),
			Notifications: os.Getenv("NOTIFICATION_SERVICE_URL"),
			Analytics:     os.Getenv("ANALYTICS_SERVICE_URL"),
		}).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Agile Manager is running at http://localhost%s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}
