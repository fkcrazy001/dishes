package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/fkcrazy001/dishes/dishes-go/internal/httpapi"
	"github.com/fkcrazy001/dishes/dishes-go/internal/realtime"
	"github.com/fkcrazy001/dishes/dishes-go/internal/store"
	_ "modernc.org/sqlite"
)

type Config struct {
	Host      string
	Port      string
	JWTSecret string
	DBFile    string
	UploadDir string
}

type App struct {
	cfg     Config
	db      *sql.DB
	store   *store.Store
	events  *realtime.Hub
	handler http.Handler
}

func New(cfg Config) (*App, error) {
	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	if cfg.DBFile == "" {
		return nil, errors.New("DB_FILE is required")
	}
	if cfg.UploadDir == "" {
		return nil, errors.New("UPLOAD_DIR is required")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DBFile), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir upload dir: %w", err)
	}

	db, err := sql.Open("sqlite", cfg.DBFile)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetConnMaxLifetime(0)
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.Migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := store.SeedIfNeeded(ctx, db); err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}

	s := store.New(db)
	hub := realtime.NewHub()

	api := httpapi.New(httpapi.Dependencies{
		Store:      s,
		Hub:        hub,
		JWTSecret:  []byte(cfg.JWTSecret),
		UploadDir:  cfg.UploadDir,
		WebDistFS:  httpapi.EmbeddedWebDist(),
		WebIndex:   "index.html",
		UploadsURL: "/uploads/",
	})

	return &App{
		cfg:     cfg,
		db:      db,
		store:   s,
		events:  hub,
		handler: api,
	}, nil
}

func (a *App) Router() http.Handler { return a.handler }
