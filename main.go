package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/dev-perry/go-server/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	tokenSecret    string
}

type fail struct {
	Error string `json:"error"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		resHeader := w.Header()
		resHeader.Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func readyHandler(r http.ResponseWriter, _ *http.Request) {
	resHeader := r.Header()
	resHeader.Set("Content-Type", "text/plain; charset=utf-8")
	r.WriteHeader(200)
	message := "OK"
	r.Write([]byte(message))
}

func (cfg *apiConfig) metricsHandler(r http.ResponseWriter, _ *http.Request) {
	hits := cfg.fileserverHits.Load()
	r.Header().Set("Content-Type", "text/html")

	response := fmt.Sprintf(`
			<html>
  				<body>
    			<h1>Welcome, Chirpy Admin</h1>
    			<p>Chirpy has been visited %d times!</p>
  				</body>
			</html>`,
		hits)
	r.Write([]byte(response))
}

func (cfg *apiConfig) reset(res http.ResponseWriter, req *http.Request) {
	platform := os.Getenv("PLATFORM")
	if platform != "dev" {
		res.WriteHeader(403)
		return
	}
	cfg.fileserverHits.Swap(0)
	cfg.db.DeleteAllUsers(req.Context())
	res.WriteHeader(200)
	message := "OK"
	res.Write([]byte(message))
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	tokenSecret := os.Getenv("TOKEN_SECRET")
	db, _ := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		tokenSecret:    tokenSecret,
	}

	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}

	fileHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(fileHandler))
	mux.HandleFunc("GET /api/healthz", readyHandler)
	mux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.reset)
	mux.HandleFunc("POST /api/login", apiCfg.loginUser)
	mux.HandleFunc("POST /api/users", apiCfg.createUser)
	mux.HandleFunc("POST /api/chirps", apiCfg.createChirp)
	mux.HandleFunc("GET /api/chirps", apiCfg.getChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirp)

	server.ListenAndServe()
}
