package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fkcrazy001/dishes/dishes-go/internal/app"
)

func main() {
	cfg := app.Config{
		Host:      getenv("HOST", "0.0.0.0"),
		Port:      getenv("PORT", "3000"),
		JWTSecret: getenv("JWT_SECRET", "dev-secret"),
		DBFile:    getenv("DB_FILE", "./data/db.sqlite"),
		UploadDir: getenv("UPLOAD_DIR", "./data/uploads"),
	}

	srv, err := app.New(cfg)
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              cfg.Host + ":" + cfg.Port,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening: http://%s", httpSrv.Addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

