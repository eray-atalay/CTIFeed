package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const adminTokenCookie = "ctifeed_admin_token"

type adminClaims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
}

func randomJWTSecret() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(secret), nil
}

func signAdminToken(secret, username string, ttl time.Duration) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims, err := json.Marshal(adminClaims{
		Subject: username,
		Role:    "admin",
		Issued:  now.Unix(),
		Expires: now.Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claims)
	message := encodedHeader + "." + encodedClaims
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return message + "." + signature, nil
}

func verifyAdminToken(secret, token string) (*adminClaims, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || secret == "" {
		return nil, false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := mac.Sum(nil)
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(expected, provided) != 1 {
		return nil, false
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}
	var claims adminClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, false
	}
	if claims.Role != "admin" || claims.Subject == "" || claims.Expires <= time.Now().UTC().Unix() {
		return nil, false
	}
	return &claims, true
}

func (s *Server) adminTokenFromRequest(r *http.Request) (*adminClaims, bool) {
	cookie, err := r.Cookie(adminTokenCookie)
	if err != nil {
		return nil, false
	}
	return verifyAdminToken(s.cfg.AdminJWTSecret, cookie.Value)
}

func adminCookie(r *http.Request, token string, maxAge int) *http.Cookie {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	return &http.Cookie{
		Name:     adminTokenCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}

func invalidCredentialsError() error {
	return fmt.Errorf("invalid admin credentials")
}
