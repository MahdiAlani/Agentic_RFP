package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"rfp-agent/internal/database"
	"rfp-agent/internal/documents"
	"rfp-agent/internal/project"
	"rfp-agent/internal/queue"
	"rfp-agent/internal/storage"
	user "rfp-agent/internal/user"
	"rfp-agent/internal/workspace"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load("../.env")

	ctx := context.Background()

	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer db.Close()

	st, err := storage.New(ctx)
	if err != nil {
		log.Fatalf("connect to storage: %v", err)
	}

	q, err := queue.New(ctx)
	if err != nil {
		log.Fatalf("connect to queue: %v", err)
	}

	repo := user.NewRepository(db)
	svc := user.NewService(repo)
	h := user.NewHandler(svc)

	wsRepo := workspace.NewRepository(db)
	wsSvc := workspace.NewService(wsRepo)
	wsHandler := workspace.NewHandler(wsSvc)

	projRepo := project.NewRepository(db)
	projSvc := project.NewService(projRepo)
	projHandler := project.NewHandler(projSvc)

	docRepo := documents.NewRepository(db)
	docSvc := documents.NewService(docRepo, st, q)
	docHandler := documents.NewHandler(docSvc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	wsHandler.RegisterRoutes(mux)
	projHandler.RegisterRoutes(mux)
	docHandler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("listening on :8080")
	log.Fatal(srv.ListenAndServe())
}
