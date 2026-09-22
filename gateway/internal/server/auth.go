package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func requireBearerToken(expectedToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		authorizationHeaders := request.Header.Values("Authorization")
		if len(authorizationHeaders) != 1 {
			writeUnauthorized(response)
			return
		}

		scheme, receivedToken, ok := strings.Cut(authorizationHeaders[0], " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || receivedToken == "" ||
			subtle.ConstantTimeCompare([]byte(receivedToken), []byte(expectedToken)) != 1 {
			writeUnauthorized(response)
			return
		}

		sanitizedRequest := request.Clone(request.Context())
		sanitizedRequest.Header = request.Header.Clone()
		sanitizedRequest.Header.Del("Authorization")
		next.ServeHTTP(response, sanitizedRequest)
	})
}

func writeUnauthorized(response http.ResponseWriter) {
	response.Header().Set("WWW-Authenticate", `Bearer realm="gateway"`)
	writeJSON(response, http.StatusUnauthorized, map[string]string{
		"error": "unauthorized",
	})
}
