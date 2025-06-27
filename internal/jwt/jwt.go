package jwt

import (
	"errors"
	"fmt"
	"main/internal/config"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt"
)

var (
	ErrTokenExpired = errors.New("token is expired")
)

type JwtTokens struct {
	AccessToken  string
	RefreshToken string
	Env          config.Env
}

func (j *JwtTokens) CreateTokens(uuid string, username, role string) error {
	var err error
	j.AccessToken, err = j.CreateToken(uuid, username, role, j.Env.EnvMap["SECRET_TTL"], j.Env.EnvMap["SECRET"])
	if err != nil {
		return fmt.Errorf("error creating JWT token: %w", err)
	}
	j.RefreshToken, err = j.CreateToken(uuid, username, role, j.Env.EnvMap["REFRESH_TTL"], j.Env.EnvMap["SECRET_REFRESH"])
	if err != nil {
		return fmt.Errorf("error creating refresh token: %w", err)
	}
	return nil
}

func (j *JwtTokens) CreateToken(uuid string, username, role, tokenTTL, secret string) (string, error) {
	exTime, err := strconv.Atoi(tokenTTL)
	if err != nil {
		return "", fmt.Errorf("failed to parse TTL from environment: %w", err)
	}
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"sub":      uuid,
			"username": username,
			"iss":      "server",
			"role":     role,
			"aud":      "service",
			"exp":      time.Now().Add(time.Duration(exTime * int(time.Minute))).Unix(),
			"iat":      time.Now().Unix(),
		}).SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("error signing token: %w", err)
	}
	return tokenString, nil
}

func ValidateToken(t, secret string) (string, string, error) {
	token, err := jwt.Parse(t, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("no valid signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		if err.Error() == "Token is expired" {
			uuid, username, err := setClaims(token)
			if err != nil {
				return uuid, "", fmt.Errorf("failed to extract claims from expired token: %w", err)
			}
			return uuid, username, ErrTokenExpired
		}
		return "", "", fmt.Errorf("failed to parse token: %w", err)
	}
	return setClaims(token)
}

func setClaims(token *jwt.Token) (string, string, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", fmt.Errorf("invalid token")
	}
	username, ok := claims["username"].(string)
	if !ok {
		return "", "", fmt.Errorf("cant parse username from jwt token")
	}
	uuid, ok := claims["sub"].(string)
	if !ok {
		return "", "", fmt.Errorf("cant parse id from jwt token")
	}
	return uuid, username, nil
}
