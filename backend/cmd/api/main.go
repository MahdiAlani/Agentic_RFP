package main

import (
	"context"
	"log"

	"rfp-agent/internal/database"
)

func main() {
	ctx := context.Background()

	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer db.Close()

	log.Println("connected to postgres")
}
