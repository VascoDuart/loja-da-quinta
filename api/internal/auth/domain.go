// Package auth trata das sessões: login, renovação com rotação do refresh
// token e deteção de reutilização, logout e verificação do access token
// (ADR-0005, docs/security/controls.md).
//
// É o único módulo que lê ou escreve a tabela refresh_tokens. As credenciais
// são do módulo users.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

const (
	// AccessTokenTTL é a duração do access token (RF-02).
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL é a duração de uma sessão sem renovação (RF-02). Cada
	// renovação dá mais 7 dias.
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// RefreshToken é um token opaco de 32 bytes aleatórios. Só o hash vai para
// a base de dados; o valor em claro só existe no cookie do browser.
type RefreshToken struct {
	Plain string
	Hash  []byte
}

// NewRefreshToken gera um refresh token novo.
func NewRefreshToken() RefreshToken {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read nunca falha (Go 1.24+)
	plain := base64.RawURLEncoding.EncodeToString(b)
	return RefreshToken{Plain: plain, Hash: HashRefresh(plain)}
}

// HashRefresh devolve o SHA-256 de um refresh token. Um hash rápido chega:
// o token tem 256 bits aleatórios, não há dicionário que o adivinhe.
func HashRefresh(plain string) []byte {
	h := sha256.Sum256([]byte(plain))
	return h[:]
}

// Session é o resultado de um login ou de uma renovação.
type Session struct {
	AccessToken string
	ExpiresIn   time.Duration
	User        users.User
	Refresh     RefreshToken
}
