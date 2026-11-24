// Package users gere os utilizadores internos (gestor e funcionário) e as
// suas credenciais. É o único módulo que lê ou escreve a tabela users.
package users

import (
	"strings"
	"time"
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
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUser são os dados para criar um utilizador.
type NewUser struct {
	Email    string
	Name     string
	Role     identity.Role
	Password string
}

// Update são as alterações a um utilizador. Os campos a nil ficam como estão.
type Update struct {
	Name     *string
	Role     *identity.Role
	Active   *bool
	Password *string
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
		return &ValidationError{Field: "email", Message: "O e-mail não é válido."}
	}
	if err := validateName(n.Name); err != nil {
		return err
	}
	if !n.Role.Valid() {
		return &ValidationError{Field: "role", Message: "O papel tem de ser gestor ou funcionário."}
	}
	return validatePassword(n.Password)
}

// Validate verifica uma alteração. Tem de mudar alguma coisa.
func (u Update) Validate() error {
	if u.Name == nil && u.Role == nil && u.Active == nil && u.Password == nil {
		return &ValidationError{Field: "", Message: "Não há nada para alterar."}
	}
	if u.Name != nil {
		if err := validateName(*u.Name); err != nil {
			return err
		}
	}
	if u.Role != nil && !u.Role.Valid() {
		return &ValidationError{Field: "role", Message: "O papel tem de ser gestor ou funcionário."}
	}
	if u.Password != nil {
		return validatePassword(*u.Password)
	}
	return nil
}

// EndsSessions diz se a alteração tem de terminar as sessões do utilizador:
// sem conta ativa, com outro papel ou com outra palavra-passe, as sessões
// abertas não podem continuar.
func (u Update) EndsSessions(before User) bool {
	return (u.Active != nil && !*u.Active) ||
		(u.Role != nil && *u.Role != before.Role) ||
		u.Password != nil
}

func validateName(name string) error {
	if name = strings.TrimSpace(name); name == "" || utf8.RuneCountInString(name) > 100 {
		return &ValidationError{Field: "name", Message: "O nome tem de ter entre 1 e 100 caracteres."}
	}
	return nil
}

func validatePassword(pw string) error {
	if l := utf8.RuneCountInString(pw); l < MinPasswordLen || l > MaxPasswordLen {
		return &ValidationError{Field: "password", Message: "A palavra-passe tem de ter entre 12 e 128 caracteres."}
	}
	return nil
}
