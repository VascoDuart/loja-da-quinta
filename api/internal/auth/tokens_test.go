package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

func newKey(t *testing.T, kid string) (string, string) {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	_, _ = rand.Read(seed)
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	return kid + ":" + base64.StdEncoding.EncodeToString(seed), kid + ":" + base64.StdEncoding.EncodeToString(pub)
}

func TestIssueAndVerify(t *testing.T) {
	signing, _ := newKey(t, "k1")
	j, err := NewJWT(signing, "")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	want := identity.Principal{UserID: uuid.New(), Role: identity.RoleGestor}
	token, err := j.Issue(want, now)
	if err != nil {
		t.Fatal(err)
	}

	got, err := j.Verify(token, now.Add(AccessTokenTTL-time.Second))
	if err != nil || got != want {
		t.Fatalf("Verify = %+v, %v", got, err)
	}
	if _, err := j.Verify(token, now.Add(AccessTokenTTL+time.Second)); !errors.Is(err, identity.ErrTokenExpired) {
		t.Errorf("depois de 15 minutos devia dar ErrTokenExpired, deu %v", err)
	}

	parsed, _, _ := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if parsed.Header["kid"] != "k1" || parsed.Header["alg"] != "EdDSA" {
		t.Errorf("cabeçalho inesperado: %v", parsed.Header)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	signing, _ := newKey(t, "k1")
	j, _ := NewJWT(signing, "")
	other, _ := newKey(t, "k1")
	impostor, _ := NewJWT(other, "")
	now := time.Now()
	p := identity.Principal{UserID: uuid.New(), Role: identity.RoleFuncionario}
	good, _ := j.Issue(p, now)

	// Trocar o papel no payload sem voltar a assinar.
	parts := strings.Split(good, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(string(payload), "funcionario", "gestor", 1)))
	tampered := strings.Join(parts, ".")

	// alg "none": sem assinatura.
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": p.UserID.String(), "role": "gestor", "iss": issuer, "aud": audience,
		"iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	fromImpostor, _ := impostor.Issue(p, now)

	unknownKID, _ := newKey(t, "k9")
	j9, _ := NewJWT(unknownKID, "")
	withUnknownKID, _ := j9.Issue(p, now)

	for name, token := range map[string]string{
		"payload alterado":         tampered,
		"alg none":                 none,
		"assinado por outra chave": fromImpostor,
		"kid desconhecido":         withUnknownKID,
		"lixo":                     "nao.e.um.jwt",
		"vazio":                    "",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := j.Verify(token, now); !errors.Is(err, identity.ErrUnauthenticated) {
				t.Errorf("devia dar ErrUnauthenticated, deu %v", err)
			}
		})
	}
}

func TestKeyRotation(t *testing.T) {
	oldSigning, oldPublic := newKey(t, "antiga")
	newSigning, _ := newKey(t, "nova")
	oldJWT, _ := NewJWT(oldSigning, "")
	now := time.Now()
	p := identity.Principal{UserID: uuid.New(), Role: identity.RoleGestor}
	issuedBefore, _ := oldJWT.Issue(p, now)

	rotated, err := NewJWT(newSigning, oldPublic)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rotated.Verify(issuedBefore, now); err != nil {
		t.Errorf("um token da chave antiga devia continuar válido até expirar: %v", err)
	}
	withoutOld, _ := NewJWT(newSigning, "")
	if _, err := withoutOld.Verify(issuedBefore, now); err == nil {
		t.Error("sem a chave antiga em JWT_VERIFY_KEYS, o token antigo não devia passar")
	}
}

func TestNewJWTRejectsBadKeys(t *testing.T) {
	_, pub := newKey(t, "k1")
	for name, c := range map[string][2]string{
		"sem kid":                 {"semdoispontos", ""},
		"base64 inválido":         {"k1:***", ""},
		"tamanho errado":          {"k1:" + base64.StdEncoding.EncodeToString([]byte("curta")), ""},
		"chave pública inválida":  {func() string { s, _ := newKey(t, "k1"); return s }(), "k2:abc"},
		"kid repetido na rotação": {func() string { s, _ := newKey(t, "k1"); return s }(), pub},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewJWT(c[0], c[1]); err == nil {
				t.Error("devia recusar a chave")
			}
		})
	}
}

func TestGenerateKeyRoundTrip(t *testing.T) {
	signing, public := GenerateKey(time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC))
	if !strings.HasPrefix(signing, "2026-11-24-") {
		t.Errorf("o kid devia começar pela data: %s", signing)
	}
	j, err := NewJWT(signing, "")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := newKey(t, "outra")
	verifier, err := NewJWT(other, public)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := j.Issue(identity.Principal{UserID: uuid.New(), Role: identity.RoleGestor}, time.Now())
	if _, err := verifier.Verify(token, time.Now()); err != nil {
		t.Errorf("a chave pública gerada devia verificar os tokens da chave privada: %v", err)
	}
}
