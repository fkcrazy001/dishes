package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const ctxUserIDKey ctxKey = "userId"

type jwtClaims struct {
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}

func (a *API) signToken(userID string) (string, error) {
	now := time.Now()
	claims := jwtClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(a.deps.JWTSecret)
}

func (a *API) verifyToken(raw string) (string, error) {
	token := strings.TrimSpace(raw)
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimPrefix(token, "bearer ")
	parsed, err := jwt.ParseWithClaims(token, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		return a.deps.JWTSecret, nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := parsed.Claims.(*jwtClaims)
	if !ok || !parsed.Valid {
		return "", jwt.ErrTokenInvalidClaims
	}
	if claims.UserID == "" {
		return "", jwt.ErrTokenInvalidClaims
	}
	return claims.UserID, nil
}

func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
			return
		}
		userID, err := a.verifyToken(auth)
		if err != nil {
			a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxUserIDKey).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

