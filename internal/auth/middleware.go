package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

type JWTMiddleware struct {
	issuer string
	jwksfn keyfunc.Keyfunc
}

type cognitoClaims struct {
	jwt.RegisteredClaims
	TokenUse string   `json:"token_use"`
	Groups   []string `json:"cognito:groups"`
}

func NewJWTMiddleware(region, userPoolID string) (*JWTMiddleware, error) {
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, userPoolID)
	jwksURL := issuer + "/.well-known/jwks.json"

	k, err := keyfunc.NewDefaultCtx(context.Background(), []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("auth: fetch JWKS from %s: %w", jwksURL, err)
	}

	return &JWTMiddleware{issuer: issuer, jwksfn: k}, nil
}

// Require returns a chi-compatible middleware that validates a Cognito Bearer JWT
// and checks that the token carries the given role in cognito:groups.
// Missing or invalid tokens → 401. Valid token without the role → 403.
// A nil receiver passes all requests through (useful in tests that don't exercise protected routes).
func (m *JWTMiddleware) Require(role string) func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := extractBearer(r)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			claims := &cognitoClaims{}
			_, err = jwt.ParseWithClaims(raw, claims, m.jwksfn.Keyfunc,
				jwt.WithIssuer(m.issuer),
				jwt.WithExpirationRequired(),
				jwt.WithValidMethods([]string{"RS256"}),
			)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			if claims.TokenUse != "access" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			for _, g := range claims.Groups {
				if g == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeJSONError(w, http.StatusForbidden, "forbidden")
		})
	}
}

func extractBearer(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", fmt.Errorf("missing Authorization header")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", fmt.Errorf("malformed Authorization header")
	}
	return parts[1], nil
}
