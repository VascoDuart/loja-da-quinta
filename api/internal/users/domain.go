// Package users gere os utilizadores internos (gestor e funcionário) e as
// suas credenciais. É o único módulo que lê ou escreve a tabela users.
package users

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// User é um utilizador interno.
type User struct {
	ID           uuid.UUID
	Email        string
	Name         string
	Role         identity.Role
	Active       bool
	PasswordHash string
}

// NewUser são os dados para criar um utilizador.
type NewUser struct {
	Email    string
	Name     string
	Role     identity.Role
	Password string
}

// Política de palavras-passe (ASVS 2.1): pelo menos 12 caracteres, até 128,
// sem regras de composição nem expiração forçada.
const (
	MinPasswordLen = 12
	MaxPasswordLen = 128
)

// NormalizeEmail devolve o e-mail sem espaços e em minúsculas: é a forma
// guardada na base de dados e a chave do limite de tentativas.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Validate verifica os dados de um utilizador novo. O e-mail já tem de vir
// normalizado.
func (n NewUser) Validate() error {
	at := strings.IndexByte(n.Email, '@')
	if len(n.Email) < 3 || len(n.Email) > 254 || at < 1 || at == len(n.Email)-1 || strings.ContainsAny(n.Email, " \t\r\n") {
		return &ValidationError{Field: "email", Message: "e-mail inválido"}
	}
	if name := strings.TrimSpace(n.Name); name == "" || utf8.RuneCountInString(name) > 100 {
		return &ValidationError{Field: "name", Message: "o nome tem de ter entre 1 e 100 caracteres"}
	}
	if !n.Role.Valid() {
		return &ValidationError{Field: "role", Message: "o papel tem de ser gestor ou funcionario"}
	}
	if l := utf8.RuneCountInString(n.Password); l < MinPasswordLen || l > MaxPasswordLen {
		return &ValidationError{Field: "password", Message: "a palavra-passe tem de ter entre 12 e 128 caracteres"}
	}
	return nil
}
