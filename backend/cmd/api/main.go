package main

import (
	"context"
	"log"
	"net/http"

	"rfp-agent/internal/database"
	user "rfp-agent/internal/User"
)

func main() {
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

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
