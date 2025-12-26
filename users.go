package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dev-perry/go-server/internal/auth"
	"github.com/dev-perry/go-server/internal/database"
	"github.com/google/uuid"
)

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type AuthSuccessResponse struct {
	User
	Token string `json:"token"`
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

func (cfg *apiConfig) loginUser(w http.ResponseWriter, r *http.Request) {
	var loginRequest struct {
		Password string `json:"password"`
		Email    string `json:"email"`
		Expires  *int   `json:"expires_in_seconds"`
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
		var duration time.Duration

		if loginRequest.Expires != nil {
			seconds := fmt.Sprintf("%vs", loginRequest.Expires)
			s, _ := time.ParseDuration(seconds)
			duration = s
		} else {
			seconds, _ := time.ParseDuration("1h")
			duration = seconds
		}
		token, tokenErr := auth.MakeJWT(dbUser.ID, cfg.tokenSecret, duration)
		if tokenErr != nil {
			w.WriteHeader(500)
			w.Write([]byte("Unable to generate user token"))
			return
		}
		authResponse := AuthSuccessResponse{
			User{
				ID:        dbUser.ID,
				Email:     dbUser.Email,
				CreatedAt: dbUser.CreatedAt,
				UpdatedAt: dbUser.UpdatedAt,
			},
			token,
		}

		response, _ := json.Marshal(authResponse)
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
