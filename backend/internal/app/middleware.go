package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.Status(204)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (a *App) ginAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := a.authedUserID(c.Request.Context(), c.GetHeader("Authorization"))
		if err != nil {
			c.JSON(401, apiErrorBody{Message: "未登录或登录已失效"})
			c.Abort()
			return
		}
		c.Set("userID", userID)
		c.Next()
	}
}

func (a *App) ginAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := mustUserID(c)
		u, ok := a.Store.FindUserByID(userID)
		if !ok || strings.TrimSpace(u.Username) != "system" {
			c.JSON(403, apiErrorBody{Message: "无权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func mustUserID(c *gin.Context) uint64 {
	v, ok := c.Get("userID")
	if !ok {
		return 0
	}
	id, _ := v.(uint64)
	return id
}

func (a *App) authedUserID(ctx context.Context, authHeader string) (uint64, error) {
	token := strings.TrimSpace(authHeader)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return 0, errors.New("missing token")
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("invalid alg")
		}
		return a.JWTSecret, nil
	})
	if err != nil || !parsed.Valid {
		return 0, errors.New("invalid token")
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("invalid claims")
	}
	sub, ok := claims["sub"]
	if !ok {
		return 0, errors.New("missing sub")
	}

	var userID uint64
	switch v := sub.(type) {
	case float64:
		if v < 0 {
			return 0, errors.New("invalid sub")
		}
		userID = uint64(v)
	case string:
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, errors.New("invalid sub")
		}
		userID = parsed
	default:
		return 0, errors.New("invalid sub")
	}

	expAny, ok := claims["exp"]
	if ok {
		switch exp := expAny.(type) {
		case float64:
			if time.Now().Unix() > int64(exp) {
				return 0, errors.New("expired")
			}
		case json.Number:
			n, _ := exp.Int64()
			if time.Now().Unix() > n {
				return 0, errors.New("expired")
			}
		}
	}
	_ = ctx
	return userID, nil
}
