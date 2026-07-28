package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"rfp-agent/internal/answers"
	"rfp-agent/internal/auth"
	"rfp-agent/internal/chat"
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

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is not set")
	}

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

	ansRepo := answers.NewRepository(db)
	ansSvc := answers.NewService(ansRepo, q)
	ansHandler := answers.NewHandler(ansSvc)

	chatRepo := chat.NewRepository(db)
	chatSvc := chat.NewService(chatRepo, q)
	chatHandler := chat.NewHandler(chatSvc)

	authHandler := auth.NewHandler(svc, secret)

	// Protected routes require a valid Bearer token (checked by auth.Middleware).
	protected := http.NewServeMux()
	h.RegisterRoutes(protected)
	wsHandler.RegisterRoutes(protected)
	projHandler.RegisterRoutes(protected)
	docHandler.RegisterRoutes(protected)
	ansHandler.RegisterRoutes(protected)
	chatHandler.RegisterRoutes(protected)
	authHandler.RegisterProtectedRoutes(protected)

	// Public routes (login/register) sit on the root mux; everything else falls
	// through the "/" catch-all into the auth-protected mux.
	mux := http.NewServeMux()
	authHandler.RegisterRoutes(mux)
	mux.Handle("/", auth.Middleware(secret)(protected))

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      logRequests(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("listening on :8080")
	log.Fatal(srv.ListenAndServe())
}

// statusWriter captures the response status code for logging.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests logs every request: method, path, status, and duration.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}
