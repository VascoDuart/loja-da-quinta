package auth

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/httpx"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/logging"
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
		return gen.Login401ApplicationProblemPlusJSONResponse(problem(ctx, http.StatusUnauthorized, "E-mail ou palavra-passe incorretos.")), nil
	case errors.Is(err, ErrTooManyAttempts):
		return gen.Login429ApplicationProblemPlusJSONResponse(problem(ctx, http.StatusTooManyRequests,
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
				Body:    problem(ctx, http.StatusUnauthorized, "A sessão terminou. Inicia sessão de novo."),
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
		return gen.GetMe401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: gen.ProblemApplicationProblemPlusJSONResponse(problem(ctx, http.StatusUnauthorized, "É preciso iniciar sessão."))}, nil
	}
	u, err := h.svc.Me(ctx, p)
	if errors.Is(err, identity.ErrUnauthenticated) {
		return gen.GetMe401ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: gen.ProblemApplicationProblemPlusJSONResponse(problem(ctx, http.StatusUnauthorized, "É preciso iniciar sessão."))}, nil
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

// Authorize é a função de autenticação do validador OpenAPI. Os papéis com
// acesso vêm da lista do bearerAuth em cada operação; uma lista vazia (o
// valor por omissão da spec) não deixa ninguém entrar.
func Authorize(_ context.Context, in *openapi3filter.AuthenticationInput) error {
	if in.SecuritySchemeName != "bearerAuth" {
		return identity.ErrForbidden
	}
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

func problem(ctx context.Context, status int, detail string) gen.Problem {
	p := gen.Problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: &detail}
	if id := logging.RequestID(ctx); id != "" {
		p.Instance = &id
	}
	return p
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
