package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

const testRole = "urbanpetr_admin"

// newTestMiddleware builds a JWTMiddleware backed by an in-memory RSA key
// and returns the middleware alongside a signing function for test JWTs.
func newTestMiddleware(t *testing.T) (*JWTMiddleware, func(claims cognitoClaims) string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	// Build an in-memory JWKS with the public key.
	jwkKey, err := jwkset.NewJWKFromKey(priv.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: "test-kid"},
	})
	if err != nil {
		t.Fatalf("build JWK: %v", err)
	}
	store := jwkset.NewMemoryStorage()
	if err := store.KeyWrite(t.Context(), jwkKey); err != nil {
		t.Fatalf("write JWK: %v", err)
	}

	// Serve the JWKS over HTTP so keyfunc can fetch it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		set, err := store.Marshal(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)

	issuer := "https://cognito-idp.eu-central-1.amazonaws.com/test-pool"
	k, err := keyfunc.NewDefaultCtx(t.Context(), []string{srv.URL})
	if err != nil {
		t.Fatalf("build keyfunc: %v", err)
	}

	m := &JWTMiddleware{issuer: issuer, jwksfn: k}

	sign := func(claims cognitoClaims) string {
		if claims.RegisteredClaims.Issuer == "" {
			claims.RegisteredClaims.Issuer = issuer
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "test-kid"
		s, err := tok.SignedString(priv)
		if err != nil {
			t.Fatalf("sign JWT: %v", err)
		}
		return s
	}

	return m, sign
}

func validClaims() cognitoClaims {
	return cognitoClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TokenUse: "access",
		Groups:   []string{testRole},
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequire_ValidToken(t *testing.T) {
	m, sign := newTestMiddleware(t)
	token := sign(validClaims())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	m.Require(testRole)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

func TestRequire_MissingHeader(t *testing.T) {
	m, _ := newTestMiddleware(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	m.Require(testRole)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestRequire_MalformedHeader(t *testing.T) {
	m, _ := newTestMiddleware(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "NotBearer token")
	rec := httptest.NewRecorder()

	m.Require(testRole)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestRequire_ExpiredToken(t *testing.T) {
	m, sign := newTestMiddleware(t)

	claims := validClaims()
	claims.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	token := sign(claims)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	m.Require(testRole)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestRequire_WrongRole(t *testing.T) {
	m, sign := newTestMiddleware(t)

	claims := validClaims()
	claims.Groups = []string{"some_other_role"}
	token := sign(claims)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	m.Require(testRole)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", rec.Code)
	}
}

func TestRequire_WrongTokenUse(t *testing.T) {
	m, sign := newTestMiddleware(t)

	claims := validClaims()
	claims.TokenUse = "id"
	token := sign(claims)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	m.Require(testRole)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestRequire_WrongIssuer(t *testing.T) {
	m, sign := newTestMiddleware(t)

	claims := validClaims()
	claims.RegisteredClaims.Issuer = "https://evil.example.com/wrong-pool"
	token := sign(claims)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	m.Require(testRole)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestExtractBearer(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		wantErr bool
		wantTok string
	}{
		{"empty", "", true, ""},
		{"no scheme", "justtoken", true, ""},
		{"wrong scheme", "Basic dXNlcjpwYXNz", true, ""},
		{"valid lower", "bearer mytoken", false, "mytoken"},
		{"valid upper", "Bearer mytoken", false, "mytoken"},
		{"valid mixed", "BEARER mytoken", false, "mytoken"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			tok, err := extractBearer(req)
			if (err != nil) != tc.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tc.wantErr, err)
			}
			if tok != tc.wantTok {
				t.Errorf("want token %q, got %q", tc.wantTok, tok)
			}
		})
	}
}
