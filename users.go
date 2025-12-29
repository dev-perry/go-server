package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/dev-perry/go-server/internal/auth"
	"github.com/dev-perry/go-server/internal/database"
	"github.com/google/uuid"
)

type credsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Email       string    `json:"email"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

type UpdatedUserRes struct {
	ID        uuid.UUID `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type AuthSuccessResponse struct {
	User
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	user := credsRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&user)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Something went wrong"))
		return
	}
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
		tokenDur, _ := time.ParseDuration("1h")
		refDur, _ := time.ParseDuration("1440h")

		duration := tokenDur
		token, tokenErr := auth.MakeJWT(dbUser.ID, cfg.tokenSecret, duration)
		refreshToken, refTokenErr := auth.MakeRefreshToken()
		if tokenErr != nil || refTokenErr != nil {
			w.WriteHeader(500)
			w.Write([]byte("Unable to generate user tokens"))
			return
		}

		refreshParams := database.CreateRefreshTokenParams{
			Token:     refreshToken,
			UserID:    dbUser.ID,
			ExpiresAt: time.Now().Add(refDur),
		}

		rToken, rTokeErr := cfg.db.CreateRefreshToken(r.Context(), refreshParams)

		if rTokeErr != nil {
			w.WriteHeader(500)
			w.Write([]byte("Unable to generate user tokens"))
			return
		}

		authResponse := AuthSuccessResponse{
			RefreshToken: rToken.Token,
			Token:        token,
			User: User{
				ID:          dbUser.ID,
				Email:       dbUser.Email,
				CreatedAt:   dbUser.CreatedAt,
				UpdatedAt:   dbUser.UpdatedAt,
				IsChirpyRed: dbUser.IsChirpyRed.Bool,
			},
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

func (cfg *apiConfig) updateUser(w http.ResponseWriter, r *http.Request) {
	token, tokenErr := auth.GetBearerToken(r.Header)
	if tokenErr != nil {
		w.WriteHeader(401)
		return
	}
	uid, authErr := auth.ValidateJWT(token, cfg.tokenSecret)
	if authErr != nil {
		w.WriteHeader(401)
		w.Write([]byte("Unauthorized"))
		return
	}
	decoder := json.NewDecoder(r.Body)
	req := credsRequest{}
	jsonErr := decoder.Decode(&req)
	if jsonErr != nil {
		w.WriteHeader(500)
		log.Printf("Encountered error in JSON decoding: %s", jsonErr)
		return
	}

	newPass, hashErr := auth.HashPassword(req.Password)
	if hashErr != nil {
		w.WriteHeader(500)
		log.Printf("Encountered error when creating has for updated password: %s", hashErr)
		return
	}

	updateParams := database.UpdateUserCredentialsParams{
		ID:             uid,
		Email:          req.Email,
		HashedPassword: newPass,
	}

	res, dbErr := cfg.db.UpdateUserCredentials(r.Context(), updateParams)
	if dbErr != nil {
		w.WriteHeader(500)
		log.Printf("Encountered error when updating user record: %s", dbErr)
		return
	}
	response := UpdatedUserRes{
		ID:        res.ID,
		UpdatedAt: res.UpdatedAt,
		Email:     res.Email,
	}
	json, _ := json.Marshal(response)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(json)
}
