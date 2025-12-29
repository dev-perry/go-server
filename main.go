package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/dev-perry/go-server/internal/auth"
	"github.com/dev-perry/go-server/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	tokenSecret    string
	polkaKey       string
}

type fail struct {
	Error string `json:"error"`
}

type RefreshTokenResponse struct {
	Token string `json:"token"`
}

type polkaRequest struct {
	Event string `json:"event"`
	Data  struct {
		UserId uuid.UUID `json:"user_id"`
	} `json:"data"`
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

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	hits := cfg.fileserverHits.Load()
	w.Header().Set("Content-Type", "text/html")

	response := fmt.Sprintf(`
			<html>
  				<body>
    			<h1>Welcome, Chirpy Admin</h1>
    			<p>Chirpy has been visited %d times!</p>
  				</body>
			</html>`,
		hits)
	w.Write([]byte(response))
}

func (cfg *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")
	if platform != "dev" {
		w.WriteHeader(403)
		return
	}
	cfg.fileserverHits.Swap(0)
	cfg.db.DeleteAllUsers(r.Context())
	w.WriteHeader(200)
	message := "OK"
	w.Write([]byte(message))
}

func (cfg *apiConfig) refreshToken(w http.ResponseWriter, r *http.Request) {
	bearer, headErr := auth.GetBearerToken(r.Header)
	if headErr != nil {
		w.WriteHeader(403)
		w.Write([]byte("Unathorized"))
		return
	}

	uid, refreshErr := cfg.db.GetUserFromRefreshToken(r.Context(), bearer)
	if refreshErr != nil {
		w.WriteHeader(401)
		return
	}
	expires, _ := time.ParseDuration("1h")
	token, tokenErr := auth.MakeJWT(uid, cfg.tokenSecret, expires)

	if tokenErr != nil {
		w.WriteHeader(500)
		w.Write([]byte("Something went wrong"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := RefreshTokenResponse{
		token,
	}

	resBuff, _ := json.Marshal(response)
	w.WriteHeader(200)
	w.Write(resBuff)
}

func (cfg *apiConfig) revokeToken(w http.ResponseWriter, r *http.Request) {
	bearer, headErr := auth.GetBearerToken(r.Header)
	if headErr != nil {
		w.WriteHeader(403)
		w.Write([]byte("Unathorized"))
		return
	}
	revokeErr := cfg.db.RevokeRefreshToken(r.Context(), bearer)
	if revokeErr != nil {
		w.WriteHeader(500)
		w.Write([]byte("Something went wrong"))
		return
	}
	w.WriteHeader(204)

}

func (cfg *apiConfig) polkaHandler(w http.ResponseWriter, r *http.Request) {
	apiKey, keyErr := auth.GetAPIKey(r.Header)
	if keyErr != nil || apiKey != cfg.polkaKey {
		w.WriteHeader(401)
		return
	}
	polka := polkaRequest{}
	decoder := json.NewDecoder(r.Body)
	if jsonErr := decoder.Decode(&polka); jsonErr != nil {
		w.WriteHeader(500)
		return
	}

	event := polka.Event
	userId := polka.Data.UserId

	if event == "user.upgraded" {
		upgradeErr := cfg.db.UpgradeUser(r.Context(), userId)

		if upgradeErr != nil {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(204)
		return
	}
	w.WriteHeader(204)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	tokenSecret := os.Getenv("TOKEN_SECRET")
	polkaKey := os.Getenv("POLKA_KEY")

	db, _ := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		tokenSecret:    tokenSecret,
		polkaKey:       polkaKey,
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
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.deleteChirp)
	mux.HandleFunc("GET /api/chirps", apiCfg.getChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.getChirp)
	mux.HandleFunc("POST /api/refresh", apiCfg.refreshToken)
	mux.HandleFunc("POST /api/revoke", apiCfg.revokeToken)
	mux.HandleFunc("PUT /api/users", apiCfg.updateUser)
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.polkaHandler)

	server.ListenAndServe()
}
