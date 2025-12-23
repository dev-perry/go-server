package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
}

type fail struct {
	Error string `json:"error"`
}

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createChirpRequest struct {
	Body   string    `json:"body"`
	UserId uuid.UUID `json:"user_id"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

var theProfane = [3]string{"kerfuffle", "sharbert", "fornax"}

func censorWord(w string) string {
	word := strings.ToLower(w)
	for _, p := range theProfane {
		if p == word {
			return "****"
		}
	}
	return w
}

func filterProfanity(input string) string {
	words := strings.Fields(input)
	for i, w := range words {
		words[i] = censorWord(w)
	}
	cleanedInput := strings.Join(words, " ")
	return cleanedInput
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		resHeader := w.Header()
		resHeader.Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	user := createUserRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&user)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Something went wrong"))
		return
	}
	log.Printf("Received: %v", user)
	if user.Email == "" {
		w.WriteHeader(400)
		w.Write([]byte("User email is required"))
		return
	}
	hashPass, passErr := auth.HashPassword(user.Password)
	if passErr != nil {
		log.Fatal(passErr)
		w.WriteHeader(500)
		w.Write([]byte("Something went wrong"))
		return
	}
	createUser := database.CreateUserParams{
		Email:          user.Email,
		HashedPassword: hashPass,
	}
	newUser, dbErr := cfg.db.CreateUser(r.Context(), createUser)

	if dbErr != nil {
		log.Printf("Error %v", dbErr)
		w.WriteHeader(500)
		w.Write([]byte("Unable to create new user"))
		return
	}

	finalUser := User{
		ID:        newUser.ID,
		CreatedAt: newUser.CreatedAt,
		UpdatedAt: newUser.UpdatedAt,
		Email:     newUser.Email,
	}

	userResponse, _ := json.Marshal(finalUser)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(userResponse)
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

func (cfg *apiConfig) createChirp(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	req := createChirpRequest{}
	err := decoder.Decode(&req)
	defaultErr := fail{
		Error: "Something went wrong",
	}

	w.Header().Set("Content-Type", "application/json")
	log.Printf("Received the following chirp: %v", req)

	if err != nil {
		errMessage, _ := json.Marshal(defaultErr)
		w.WriteHeader(500)
		w.Write(errMessage)
		return
	}

	if len(req.Body) > 140 {
		message := "Chirp is too long"
		w.WriteHeader(400)
		w.Write([]byte(message))
		return
	} else {
		req.Body = filterProfanity(req.Body)
		insertChirp := database.CreateChirpParams{
			Body:   req.Body,
			UserID: req.UserId,
		}
		newChirp, dbErr := cfg.db.CreateChirp(r.Context(), insertChirp)
		if dbErr != nil {
			log.Printf("Datbase error %v", dbErr)
			w.WriteHeader(500)
			return
		}
		chirp := Chirp{
			ID:        newChirp.ID,
			CreatedAt: newChirp.CreatedAt,
			UpdatedAt: newChirp.UpdatedAt,
			Body:      newChirp.Body,
			UserID:    newChirp.UserID,
		}
		chirpResponse, _ := json.Marshal(chirp)
		w.WriteHeader(201)
		w.Write(chirpResponse)
	}
}

func (cfg *apiConfig) getChirps(w http.ResponseWriter, r *http.Request) {
	chirps, dbErr := cfg.db.GetAllChirps(r.Context())
	if dbErr != nil {
		w.WriteHeader(500)
		w.Write([]byte("Something went wrong"))
		return
	}
	chirpResponse := make([]Chirp, len(chirps))
	for i, c := range chirps {
		chirpResponse[i] = Chirp{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			UserID:    c.UserID,
		}
	}
	response, _ := json.Marshal(chirpResponse)
	w.Write(response)
}

func (cfg *apiConfig) getChirp(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	if chirpID == "" {
		w.WriteHeader(404)
		w.Write([]byte("Chirp ID required"))
		return
	}
	c, dbErr := cfg.db.GetChirp(r.Context(), uuid.MustParse(chirpID))
	if dbErr != nil {
		w.WriteHeader(404)
		w.Write([]byte("Something went wrong"))
		return
	}
	responseChirp := Chirp{
		ID:        c.ID,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Body:      c.Body,
		UserID:    c.UserID,
	}

	response, _ := json.Marshal(responseChirp)

	w.Write(response)
}

func (cfg *apiConfig) loginUser(w http.ResponseWriter, r *http.Request) {
	var loginRequest struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&loginRequest)
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte("Unable to decode request"))
		return
	}
	dbUser, dbErr := cfg.db.GetUserCredsByEmail(r.Context(), loginRequest.Email)
	if dbErr != nil {
		w.WriteHeader(500)
		w.Write([]byte("Something went wrong: Unable to find user."))
		return
	}
	matchPass, passErr := auth.CheckPasswordHash(loginRequest.Password, dbUser.HashedPassword)
	if passErr != nil {
		w.WriteHeader(401)
		w.Write([]byte("Incorrect email or password"))
		return
	}
	if matchPass {
		user := User{
			ID:        dbUser.ID,
			Email:     dbUser.Email,
			CreatedAt: dbUser.CreatedAt,
			UpdatedAt: dbUser.UpdatedAt,
		}
		response, _ := json.Marshal(user)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(response)
		return
	} else {
		w.WriteHeader(401)
		w.Write([]byte("Incorrect email or password"))
		return
	}
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
	db, _ := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
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
