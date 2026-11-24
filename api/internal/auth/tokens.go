package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

const (
	issuer   = "loja-da-quinta"
	audience = "loja-da-quinta-api"
)

// JWT emite e verifica access tokens assinados com Ed25519. O kid no
// cabeçalho diz que chave assinou, para se poder trocar de chave sem
// invalidar as sessões: a chave nova assina, a antiga só verifica até os
// tokens dela expirarem (15 min).
type JWT struct {
	kid     string
	signing ed25519.PrivateKey
	verify  map[string]ed25519.PublicKey
}

var _ Tokens = (*JWT)(nil)

// NewJWT lê a chave de assinatura ("kid:seed", com a seed de 32 bytes em
// base64) e, opcionalmente, chaves públicas antigas que ainda verificam
// ("kid:chave-publica,kid:chave-publica").
func NewJWT(signingKey, verifyKeys string) (*JWT, error) {
	kid, seed, err := parseKey(signingKey, ed25519.SeedSize)
	if err != nil {
		return nil, fmt.Errorf("JWT_SIGNING_KEY: %w", err)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	j := &JWT{kid: kid, signing: priv, verify: map[string]ed25519.PublicKey{kid: priv.Public().(ed25519.PublicKey)}}

	for _, entry := range strings.Split(verifyKeys, ",") {
		if entry = strings.TrimSpace(entry); entry == "" {
			continue
		}
		oldKID, pub, err := parseKey(entry, ed25519.PublicKeySize)
		if err != nil {
			return nil, fmt.Errorf("JWT_VERIFY_KEYS: %w", err)
		}
		if _, dup := j.verify[oldKID]; dup {
			return nil, fmt.Errorf("JWT_VERIFY_KEYS: kid %q repetido", oldKID)
		}
		j.verify[oldKID] = ed25519.PublicKey(pub)
	}
	return j, nil
}

// NewEphemeralJWT cria uma chave aleatória que só dura enquanto o processo
// corre. Só para desenvolvimento: ao reiniciar a API, os access tokens
// deixam de valer (a renovação continua a funcionar).
func NewEphemeralJWT() *JWT {
	seed := make([]byte, ed25519.SeedSize)
	_, _ = rand.Read(seed)
	j, _ := NewJWT("dev:"+base64.StdEncoding.EncodeToString(seed), "")
	return j
}

// GenerateKey devolve uma chave nova no formato de JWT_SIGNING_KEY e a
// chave pública correspondente, no formato de JWT_VERIFY_KEYS.
func GenerateKey(now time.Time) (signing, public string) {
	seed := make([]byte, ed25519.SeedSize)
	_, _ = rand.Read(seed)
	suffix := make([]byte, 3)
	_, _ = rand.Read(suffix)
	kid := now.UTC().Format("2006-01-02") + "-" + hex.EncodeToString(suffix)
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	return kid + ":" + base64.StdEncoding.EncodeToString(seed), kid + ":" + base64.StdEncoding.EncodeToString(pub)
}

func parseKey(s string, size int) (string, []byte, error) {
	kid, b64, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok || kid == "" || len(kid) > 64 {
		return "", nil, errors.New("formato esperado kid:base64")
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(raw) != size {
		return "", nil, fmt.Errorf("a chave do kid %q tem de ter %d bytes em base64", kid, size)
	}
	return kid, raw, nil
}

type claims struct {
	Role identity.Role `json:"role"`
	jwt.RegisteredClaims
}

// Issue emite um access token de 15 minutos.
func (j *JWT) Issue(p identity.Principal, now time.Time) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims{
		Role: p.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			Subject:   p.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	})
	t.Header["kid"] = j.kid
	return t.SignedString(j.signing)
}

// Verify valida a assinatura, o algoritmo (só EdDSA: impede "alg: none" e a
// troca de algoritmo), o emissor, a audiência e a validade.
func (j *JWT) Verify(token string, now time.Time) (identity.Principal, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		key, ok := j.verify[kid]
		if !ok {
			return nil, errors.New("kid desconhecido")
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if errors.Is(err, jwt.ErrTokenExpired) {
		return identity.Principal{}, identity.ErrTokenExpired
	}
	if err != nil {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil || !c.Role.Valid() {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return identity.Principal{UserID: id, Role: c.Role}, nil
}
