package auth

import (
	"log"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
)

type ChirpyClaims struct {
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		log.Fatal(err)
		return "", err
	}

	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		log.Fatal(err)
		return match, err
	}
	return match, nil
}
