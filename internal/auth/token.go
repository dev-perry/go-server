package auth

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	claims := ChirpyClaims{
		jwt.RegisteredClaims{
			Issuer:    "chirpy",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
			Subject:   userID.String(),
		}}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString([]byte(tokenSecret))

	return ss, err
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ChirpyClaims{}, func(token *jwt.Token) (interface{}, error) {

		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(tokenSecret), nil
	})

	if err != nil {
		log.Fatalf("Error parsing token: %v", err)
		return uuid.UUID{}, err
	}

	if claims, ok := token.Claims.(*ChirpyClaims); ok && token.Valid {
		userUUID := uuid.MustParse(claims.Subject)
		return userUUID, nil
	} else {
		log.Fatal("Invalid token")
		return uuid.UUID{}, errors.New("Invalid token")
	}
}

func GetBearerToken(headers http.Header) (string, error) {
	authorization := headers.Get("Authorization")
	if authorization == "" {
		return "", errors.New("No Authorization Header found")
	}
	tokenString, stringErr := strings.CutPrefix(authorization, "Bearer ")
	if !stringErr {
		return "", errors.New("Authorization Header is not in the correct format")
	}

	log.Println("Token string:", tokenString)

	return tokenString, nil
}

// func MakeRefreshToken() (string, error) {

// }
