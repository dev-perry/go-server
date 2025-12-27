package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dev-perry/go-server/internal/auth"
	"github.com/dev-perry/go-server/internal/database"
	"github.com/google/uuid"
)

type createChirpRequest struct {
	Body string `json:"body"`
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

func (cfg *apiConfig) createChirp(w http.ResponseWriter, r *http.Request) {
	token, tokenErr := auth.GetBearerToken(r.Header)
	if tokenErr != nil {
		w.WriteHeader(403)
		w.Write([]byte("Unathorized"))
		return
	}
	decoder := json.NewDecoder(r.Body)
	req := createChirpRequest{}
	err := decoder.Decode(&req)
	defaultErr := fail{
		Error: "Something went wrong",
	}

	uid, authErr := auth.ValidateJWT(token, cfg.tokenSecret)
	if authErr != nil {
		w.WriteHeader(401)
		w.Write([]byte("Unauthorized"))
		return
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
			UserID: uid,
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
