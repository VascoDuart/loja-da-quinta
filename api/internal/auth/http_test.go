package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// authorizeWith corre o Authenticate e depois o Authorizer, como no servidor.
func authorizeWith(t *testing.T, svc *Service, header string, roles []string) error {
	t.Helper()
	var result error
	h := svc.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result = Authorizer(testJobToken)(r.Context(), &openapi3filter.AuthenticationInput{
			RequestValidationInput: &openapi3filter.RequestValidationInput{Request: r},
			SecuritySchemeName:     "bearerAuth",
			Scopes:                 roles,
		})
	}))
	r := httptest.NewRequest("GET", "/api/v1/me", nil)
	if header != "" {
		r.Header.Set("Authorization", header)
	}
	h.ServeHTTP(httptest.NewRecorder(), r)
	return result
}

func TestAuthorize(t *testing.T) {
	f := newFixture(t)
	gestor := f.login(t).AccessToken
	funcionario, _ := f.svc.tokens.Issue(identity.Principal{UserID: uuid.New(), Role: identity.RoleFuncionario}, f.now)
	expired, _ := f.svc.tokens.Issue(identity.Principal{UserID: uuid.New(), Role: identity.RoleGestor}, f.now.Add(-time.Hour))

	both := []string{"funcionario", "gestor"}
	onlyGestor := []string{"gestor"}

	cases := []struct {
		name   string
		header string
		roles  []string
		want   error
	}{
		{"sem token", "", both, identity.ErrUnauthenticated},
		{"esquema errado", "Basic abc", both, identity.ErrUnauthenticated},
		{"token inválido", "Bearer abc", both, identity.ErrUnauthenticated},
		{"token expirado", "Bearer " + expired, both, identity.ErrTokenExpired},
		{"gestor numa rota de todos", "Bearer " + gestor, both, nil},
		{"funcionário numa rota de todos", "Bearer " + funcionario, both, nil},
		{"funcionário numa rota só do gestor", "Bearer " + funcionario, onlyGestor, identity.ErrForbidden},
		{"bearer em minúsculas", "bearer " + gestor, onlyGestor, nil},
		{"operação sem papéis (por omissão)", "Bearer " + gestor, []string{}, identity.ErrForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := authorizeWith(t, f.svc, c.header, c.roles); !errors.Is(err, c.want) {
				t.Errorf("queria %v, deu %v", c.want, err)
			}
		})
	}
}

func TestAuthorizeUnknownSchemeIsDenied(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	err := Authorizer(testJobToken)(context.Background(), &openapi3filter.AuthenticationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: identityRequest(r)},
		SecuritySchemeName:     "apiKey",
		Scopes:                 []string{"gestor"},
	})
	if !errors.Is(err, identity.ErrForbidden) {
		t.Errorf("um esquema desconhecido devia ser recusado: %v", err)
	}
}

var testJobToken = strings.Repeat("j", 40)

func TestAuthorizeJobToken(t *testing.T) {
	authorize := func(configured, header string, scopes []string) error {
		r := httptest.NewRequest("POST", "/api/v1/jobs/stock-alerts", nil)
		if header != "" {
			r.Header.Set(JobTokenHeader, header)
		}
		// Uma sessão de gestor no contexto: não substitui o token dos jobs.
		return Authorizer(configured)(context.Background(), &openapi3filter.AuthenticationInput{
			RequestValidationInput: &openapi3filter.RequestValidationInput{Request: identityRequest(r)},
			SecuritySchemeName:     "jobToken",
			Scopes:                 scopes,
		})
	}
	if err := authorize(testJobToken, testJobToken, nil); err != nil {
		t.Errorf("o token certo devia entrar: %v", err)
	}
	for name, header := range map[string]string{
		"sem cabeçalho": "", "errado": "outro", "um prefixo": testJobToken[:20], "com mais um carácter": testJobToken + "x",
	} {
		if err := authorize(testJobToken, header, nil); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Errorf("%s: queria ErrUnauthenticated, deu %v", name, err)
		}
	}
	// Sem token configurado (desenvolvimento), nem um cabeçalho vazio entra.
	for _, header := range []string{"", "qualquer"} {
		if err := authorize("", header, nil); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Errorf("sem JOBS_TOKEN, %q: deu %v", header, err)
		}
	}
	if err := authorize(testJobToken, testJobToken, []string{"gestor"}); !errors.Is(err, identity.ErrForbidden) {
		t.Errorf("um jobToken com papéis é um engano na spec: %v", err)
	}
}

func identityRequest(r *http.Request) *http.Request {
	return r.WithContext(identity.With(r.Context(), identity.Principal{UserID: uuid.New(), Role: identity.RoleGestor}))
}

// publicOperations são as únicas operações sem sessão. Acrescentar uma
// operação pública obriga a mudar este teste, o que fica visível no PR.
var publicOperations = []string{"getHealth", "getReadiness", "login", "refreshSession", "logout", "getProductImage", "getPublicCatalog"}

// jobOperations são as únicas operações que se abrem com o token dos jobs em
// vez de uma sessão. Como as públicas, acrescentar uma obriga a mudar este
// teste.
var jobOperations = []string{"runStockAlerts"}

// listed compara sem maiúsculas: a spec embebida pelo oapi-codegen
// capitaliza os operationId.
func listed(list []string, id string) bool {
	return slices.ContainsFunc(list, func(p string) bool { return strings.EqualFold(p, id) })
}

// TestEveryOperationDeclaresItsAccess garante a regra "negado por omissão"
// na própria spec: cada operação é pública ou de um job (e está na respetiva
// lista acima), ou usa o bearerAuth só com papéis que existem.
func TestEveryOperationDeclaresItsAccess(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	jobScheme := spec.Components.SecuritySchemes["jobToken"]
	if jobScheme == nil || jobScheme.Value.Type != "apiKey" || jobScheme.Value.In != "header" || jobScheme.Value.Name != JobTokenHeader {
		t.Fatalf("o esquema jobToken tem de ser uma apiKey no cabeçalho %s", JobTokenHeader)
	}
	jobs := 0
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			security := spec.Security
			if op.Security != nil {
				security = *op.Security
			}
			name := method + " " + path + " (" + op.OperationID + ")"

			if len(security) == 0 {
				if !listed(publicOperations, op.OperationID) {
					t.Errorf("%s é pública mas não está em publicOperations", name)
				}
				continue
			}
			if listed(publicOperations, op.OperationID) {
				t.Errorf("%s está em publicOperations mas exige sessão", name)
			}
			if listed(jobOperations, op.OperationID) {
				jobs++
				// Só o jobToken: uma segunda alternativa abria o job por
				// outro caminho.
				roles, ok := security[0]["jobToken"]
				if len(security) != 1 || len(security[0]) != 1 || !ok || len(roles) != 0 {
					t.Errorf("%s está em jobOperations: tem de declarar só jobToken, sem papéis", name)
				}
				continue
			}
			for _, req := range security {
				for scheme, roles := range req {
					if scheme != "bearerAuth" {
						t.Errorf("%s usa o esquema %s: só as operações de jobOperations não usam o bearerAuth", name, scheme)
					}
					if len(roles) == 0 {
						t.Errorf("%s não declara papéis: ninguém lhe consegue aceder", name)
					}
					for _, role := range roles {
						if !identity.Role(role).Valid() {
							t.Errorf("%s declara um papel que não existe: %q", name, role)
						}
					}
				}
			}
		}
	}
	if jobs != len(jobOperations) {
		t.Errorf("há %d operações em jobOperations mas só %d na spec", len(jobOperations), jobs)
	}
}

func TestRefreshCookieAttributes(t *testing.T) {
	c := refreshCookie("valor")
	for _, want := range []string{"refresh_token=valor", "Path=/api/v1/auth", "Max-Age=604800", "HttpOnly", "Secure", "SameSite=Strict"} {
		if !strings.Contains(c, want) {
			t.Errorf("o cookie devia ter %q: %s", want, c)
		}
	}
	if clear := clearCookie(); !strings.Contains(clear, "Max-Age=0") || !strings.Contains(clear, "Path=/api/v1/auth") {
		t.Errorf("o cookie de logout devia apagar o refresh: %s", clear)
	}
}
