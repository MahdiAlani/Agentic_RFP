package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"rfp-agent/internal/database"
	user "rfp-agent/internal/user"
)

func main() {
	godotenv.Load("../.env")

	ctx := context.Background()

	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer db.Close()

	repo := user.NewRepository(db)
	svc := user.NewService(repo)
	h := user.NewHandler(svc)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

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
