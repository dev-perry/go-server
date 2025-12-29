package auth

import (
	"errors"
	"net/http"
	"strings"
)

func GetAPIKey(headers http.Header) (string, error) {
	authorization := headers.Get("Authorization")
	apiKey, prefixFound := strings.CutPrefix(authorization, "ApiKey ")
	if !prefixFound {
		return "", errors.New("API Key is not in the correct format")
	}

	return apiKey, nil

}
