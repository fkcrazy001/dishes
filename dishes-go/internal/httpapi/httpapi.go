package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/fkcrazy001/dishes/dishes-go/internal/realtime"
	"github.com/fkcrazy001/dishes/dishes-go/internal/store"
)

type Dependencies struct {
	Store      *store.Store
	Hub        *realtime.Hub
	JWTSecret  []byte
	UploadDir  string
	WebDistFS  fs.FS
	WebIndex   string
	UploadsURL string
}

type API struct {
	deps Dependencies
}

func New(deps Dependencies) http.Handler {
	api := &API{deps: deps}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(api.cors)
	r.Use(api.jsonHeaders)

	if deps.UploadDir != "" && deps.UploadsURL != "" {
		r.Mount(deps.UploadsURL, api.uploadsHandler())
	}

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/register", api.handleRegister)
		r.Post("/auth/login", api.handleLogin)
		r.With(api.requireAuth).Get("/me", api.handleMe)
		r.Get("/users", api.handleUsersRank)

		r.Get("/dishes", api.handleListDishes)
		r.Get("/dishes/{dishId}", api.handleGetDish)
		r.With(api.requireAuth).Post("/dishes", api.handleCreateDish)
		r.With(api.requireAuth).Delete("/dishes/{dishId}", api.handleDeleteDish)

		r.With(api.requireAuth).Post("/orders", api.handleCreateOrder)
		r.With(api.requireAuth).Get("/orders", api.handleListOrders)
		r.With(api.requireAuth).Get("/orders/{orderId}", api.handleGetOrder)
		r.With(api.requireAuth).Post("/orders/{orderId}/accept", api.handleAcceptOrder)
		r.With(api.requireAuth).Post("/orders/{orderId}/cancel", api.handleCancelOrder)
		r.With(api.requireAuth).Post("/orders/{orderId}/finish", api.handleFinishOrder)
		r.With(api.requireAuth).Post("/orders/{orderId}/review", api.handleReviewOrder)

		r.With(api.requireAuth).Get("/orders/stream", api.handleOrdersStream)

		r.Get("/ws/orders", api.handleOrdersWS)
	})

	r.NotFound(api.spaFallback(deps.WebDistFS, deps.WebIndex))

	return r
}

func (a *API) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Accept")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) jsonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") && !strings.HasPrefix(r.URL.Path, "/api/orders/stream") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) uploadsHandler() http.Handler {
	root := a.deps.UploadDir
	return http.StripPrefix(a.deps.UploadsURL, http.FileServer(http.Dir(root)))
}

func (a *API) spaFallback(dist fs.FS, index string) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(dist))

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, a.deps.UploadsURL) {
			a.writeError(w, http.StatusNotFound, "NOT_FOUND", "接口不存在", nil)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = index
		}

		f, err := dist.Open(path)
		if err == nil {
			defer f.Close()
			stat, _ := f.Stat()
			if stat != nil && !stat.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + index
		fileServer.ServeHTTP(w, r2)
	}
}

func (a *API) readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return errors.New("empty body")
	}
	return json.Unmarshal(body, v)
}

func (a *API) writeOK(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}

func (a *API) writeError(w http.ResponseWriter, status int, code, message string, details any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error": map[string]any{
			"code":    code,
			"message": message,
			"details": details,
		},
	})
}

func ensureDir(p string) error {
	return os.MkdirAll(filepath.Clean(p), 0o755)
}

func nowMilli() int64 { return time.Now().UnixMilli() }
