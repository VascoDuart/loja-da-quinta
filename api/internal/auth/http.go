package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/problems"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpx"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

// O cookie só vai para /api/v1/auth: os outros pedidos nunca o levam, por
// isso um CSRF noutro endpoint não tem cookie de sessão para abusar.
const (
	cookieName = "refresh_token"
	cookiePath = "/api/v1/auth"
)

// Endpoints implementa as operações da tag auth da OpenAPI.
type Endpoints struct{ svc *Service }

// NewEndpoints cria os handlers.
func NewEndpoints(svc *Service) *Endpoints { return &Endpoints{svc: svc} }

func (h *Endpoints) Login(ctx context.Context, req gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	sess, err := h.svc.Login(ctx, req.Body.Email, req.Body.Password, httpx.ClientIP(ctx))
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return gen.Login401ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnauthorized, "E-mail ou palavra-passe incorretos.")), nil
	case errors.Is(err, ErrTooManyAttempts):
		return gen.Login429ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusTooManyRequests,
			"Demasiadas tentativas falhadas. Espera 15 minutos e tenta de novo.")), nil
	case err != nil:
		return nil, err
	}
	return gen.Login200JSONResponse{SessionStartedJSONResponse: started(sess)}, nil
}

func (h *Endpoints) RefreshSession(ctx context.Context, req gen.RefreshSessionRequestObject) (gen.RefreshSessionResponseObject, error) {
	sess, err := h.svc.Refresh(ctx, deref(req.Params.RefreshToken))
	if errors.Is(err, ErrSessionInvalid) || errors.Is(err, ErrRefreshReused) {
		return gen.RefreshSession401ApplicationProblemPlusJSONResponse{
			SessionEndedApplicationProblemPlusJSONResponse: gen.SessionEndedApplicationProblemPlusJSONResponse{
				Body:    problems.New(ctx, http.StatusUnauthorized, "A sessão terminou. Inicia sessão de novo."),
				Headers: gen.SessionEndedResponseHeaders{SetCookie: clearCookie()},
			},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return gen.RefreshSession200JSONResponse{SessionStartedJSONResponse: started(sess)}, nil
}

func (h *Endpoints) Logout(ctx context.Context, req gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	if err := h.svc.Logout(ctx, deref(req.Params.RefreshToken)); err != nil {
		return nil, err
	}
	return gen.Logout204Response{Headers: gen.Logout204ResponseHeaders{SetCookie: clearCookie()}}, nil
}

func (h *Endpoints) GetMe(ctx context.Context, _ gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	p, ok := identity.From(ctx)
	if !ok {
		return gen.GetMe401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: gen.ProblemApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnauthorized, "É preciso iniciar sessão."))}, nil
	}
	u, err := h.svc.Me(ctx, p)
	if errors.Is(err, identity.ErrUnauthenticated) {
		return gen.GetMe401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: gen.ProblemApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnauthorized, "É preciso iniciar sessão."))}, nil
	}
	if err != nil {
		return nil, err
	}
	return gen.GetMe200JSONResponse(toUser(u)), nil
}

// --- Middleware ------------------------------------------------------------

type authErrKey struct{}

// Authenticate lê o "Authorization: Bearer" (se houver) e põe o principal
// no contexto. Não recusa nada: quem decide é o Authorize, com os papéis de
// cada operação na OpenAPI. Um token inválido fica registado no contexto
// para a resposta dizer se expirou.
func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		scheme, token, ok := strings.Cut(header, " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			ctx = context.WithValue(ctx, authErrKey{}, identity.ErrUnauthenticated)
		} else if p, err := s.Verify(token); err != nil {
			ctx = context.WithValue(ctx, authErrKey{}, err)
		} else {
			ctx = identity.With(ctx, p)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// JobTokenHeader é o cabeçalho com o token dos jobs agendados (esquema
// jobToken da OpenAPI, ADR-0015).
const JobTokenHeader = "X-Job-Token"

// Authorizer devolve a função de autenticação do validador OpenAPI, que
// decide cada operação pelo esquema que a spec lhe dá:
//
//   - bearerAuth: a sessão de um utilizador com um dos papéis da operação;
//   - jobToken: o token dos jobs, e só ele (uma sessão não serve). Sem
//     jobToken configurado, ninguém entra.
//
// Um esquema que não esteja aqui não deixa ninguém entrar.
func Authorizer(jobToken string) openapi3filter.AuthenticationFunc {
	// Compara-se o hash, para o tempo não depender nem do conteúdo nem do
	// tamanho do token.
	expected := sha256.Sum256([]byte(jobToken))
	return func(_ context.Context, in *openapi3filter.AuthenticationInput) error {
		switch in.SecuritySchemeName {
		case "bearerAuth":
			return authorizeSession(in)
		case "jobToken":
			// Um job não tem papéis: uma lista na spec é um engano.
			if len(in.Scopes) != 0 {
				return identity.ErrForbidden
			}
			got := sha256.Sum256([]byte(in.RequestValidationInput.Request.Header.Get(JobTokenHeader)))
			if subtle.ConstantTimeCompare(got[:], expected[:]) != 1 || jobToken == "" {
				return identity.ErrUnauthenticated
			}
			return nil
		default:
			return identity.ErrForbidden
		}
	}
}

// authorizeSession decide uma operação com bearerAuth. Os papéis com acesso
// vêm da lista da operação; uma lista vazia (o valor por omissão da spec) não
// deixa ninguém entrar.
func authorizeSession(in *openapi3filter.AuthenticationInput) error {
	ctx := in.RequestValidationInput.Request.Context()
	p, ok := identity.From(ctx)
	if !ok {
		if err, _ := ctx.Value(authErrKey{}).(error); err != nil {
			return err
		}
		return identity.ErrUnauthenticated
	}
	if !slices.Contains(in.Scopes, string(p.Role)) {
		return identity.ErrForbidden
	}
	return nil
}

// --- Conversões ------------------------------------------------------------

func started(s Session) gen.SessionStartedJSONResponse {
	return gen.SessionStartedJSONResponse{
		Body: gen.Session{
			AccessToken: s.AccessToken,
			TokenType:   gen.Bearer,
			ExpiresIn:   int(s.ExpiresIn.Seconds()),
			User:        toUser(s.User),
		},
		Headers: gen.SessionStartedResponseHeaders{SetCookie: refreshCookie(s.Refresh.Plain)},
	}
}

func toUser(u users.User) gen.User {
	return gen.User{Id: u.ID, Email: u.Email, Name: u.Name, Role: gen.Role(u.Role)}
}

func refreshCookie(value string) string {
	c := http.Cookie{
		Name: cookieName, Value: value, Path: cookiePath,
		MaxAge: int(RefreshTokenTTL.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	}
	return c.String()
}

func clearCookie() string {
	c := http.Cookie{Name: cookieName, Value: "", Path: cookiePath, MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
	return c.String()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
