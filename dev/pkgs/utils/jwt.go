package utils

import (
	db "api/database"
	repo "api/internal/app/repository"
	req "api/internal/app/request"

	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"os"
	"time"
)

var (
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
)

// Claims mendefinisikan struktur untuk token JWT
type Claims struct {
	UserID int32                   `json:"user_id"`
	Email  string                  `json:"email"`
	Roles  []repo.AllRoleClientRow `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

// RefreshClaims mendefinisikan struktur untuk refresh token
type RefreshClaims struct {
	UserID int32  `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// loadKey membaca dan memproses file kunci RSA
func loadKey(filename string, isPrivate bool) (interface{}, error) {
	keyBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing the key")
	}

	if isPrivate {
		if block.Type != "RSA PRIVATE KEY" {
			return nil, errors.New("invalid private key format")
		}
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}

	if block.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid public key format")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPubKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("parsed public key is not an RSA key")
	}

	return rsaPubKey, nil
}

// LoadPrivateKey memuat kunci privat dari file
func LoadPrivateKey() (*rsa.PrivateKey, error) {
	key, err := loadKey("private.pem", true)
	if err != nil {
		return nil, err
	}
	return key.(*rsa.PrivateKey), nil
}

// LoadPublicKey memuat kunci publik dari file
func LoadPublicKey() (*rsa.PublicKey, error) {
	key, err := loadKey("public.pem", false)
	if err != nil {
		return nil, err
	}
	return key.(*rsa.PublicKey), nil
}

// InitRSAKeys menginisialisasi kunci RSA
func CheckRSA() {
	privateKey, err := LoadPrivateKey()
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}
	PrivateKey = privateKey

	publicKey, err := LoadPublicKey()
	if err != nil {
		log.Fatalf("Failed to load public key: %v", err)
	}
	PublicKey = publicKey
}

// CreateToken membuat access token
func CreateToken(session req.Jwt) (string, error) {
	if PrivateKey == nil {
		return "", errors.New("private key is nil")
	}

	now := time.Now()
	claims := Claims{
		UserID: session.ID,
		Email:  session.Email,
		Roles:  session.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour * 1)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return token.SignedString(PrivateKey)
}

// CreateRefreshToken membuat refresh token
func CreateRefreshToken(session req.Jwt) (string, error) {
	if PrivateKey == nil {
		return "", errors.New("private key is nil")
	}

	now := time.Now()
	claims := RefreshClaims{
		UserID: session.ID,
		Email:  session.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return token.SignedString(PrivateKey)
}

// AutoRefreshToken memperbarui token secara otomatis
func AutoRefreshToken(userID int32) (string, error) {
	user, err := db.Queries.GetClient(context.Background(), userID)
	if err != nil {
		return "", err
	}

	role, err := db.Queries.AllRoleClient(context.Background(), user.ID)
	if err != nil {
		return "", err
	}

	jwtData := req.Jwt{
		ID:    user.ID,
		Email: user.Email,
		Role:  role,
	}

	freshJwt, err := CreateToken(jwtData)
	if err != nil {
		return "", err
	}
	return freshJwt, nil
}
