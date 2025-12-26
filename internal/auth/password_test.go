package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeJWTAndValidateJWT(t *testing.T) {
	// Setup
	tokenSecret := "test-secret-key"
	duration := time.Hour
	expectedUUID := uuid.New()

	// Create a JWT token
	token, err := MakeJWT(expectedUUID, tokenSecret, duration)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	// Validate the token and extract the UUID
	extractedUUID, err := ValidateJWT(token, tokenSecret)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	// Check that the extracted UUID matches the original
	if extractedUUID != expectedUUID {
		t.Errorf("UUID mismatch: expected %s, got %s", expectedUUID, extractedUUID)
	}
}
