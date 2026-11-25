//go:build integration

package integration

import (
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// pathParam apanha os parâmetros de caminho ({id}, {productId}…).
var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// TestAuthorizationFromSpec gera a parte negativa da matriz de autorização a
// partir da própria OpenAPI: para cada operação que não é pública,
//
//   - sem credenciais: 401 (nunca 2xx);
//   - com sessão de cada papel que não está na lista do bearerAuth: 403.
//
// A segurança é verificada antes dos parâmetros e do corpo, por isso basta
// um ID qualquer e nenhum corpo: o pedido é recusado antes de chegar ao
// handler e nada se altera. Um endpoint novo fica coberto sem escrever um
// teste; nos testes de cada issue só entra o caso positivo.
func TestAuthorizationFromSpec(t *testing.T) {
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatal(err)
	}

	e := newEnv(t)
	e.createUser(t, "gestor@quinta.pt", identity.RoleGestor, pw)
	e.createUser(t, "funcionario@quinta.pt", identity.RoleFuncionario, pw)
	srv := e.server(t)
	gestor, _ := login(t, srv, "gestor@quinta.pt")
	funcionario, _ := login(t, srv, "funcionario@quinta.pt")
	tokens := map[identity.Role]string{identity.RoleGestor: gestor, identity.RoleFuncionario: funcionario}

	paths := spec.Paths.Map()
	keys := make([]string, 0, len(paths))
	for p := range paths {
		keys = append(keys, p)
	}
	sort.Strings(keys)

	checked := 0
	for _, path := range keys {
		target := pathParam.ReplaceAllString(path, "00000000-0000-4000-8000-000000000000")
		for method, op := range paths[path].Operations() {
			security := spec.Security
			if op.Security != nil {
				security = *op.Security
			}
			if len(security) == 0 {
				continue // pública por decisão explícita (TestEveryOperationDeclaresItsAccess)
			}
			name := method + " " + path

			t.Run(name+" sem credenciais", func(t *testing.T) {
				r := call(t, srv, method, target, nil, nil)
				if r.status != http.StatusUnauthorized {
					t.Errorf("queria 401, deu %d %v", r.status, r.body)
				}
			})
			checked++

			// Os papéis com acesso estão na lista do bearerAuth; todos os
			// outros têm de ser recusados com 403.
			var allowed []string
			usesBearer := false
			for _, req := range security {
				if roles, ok := req["bearerAuth"]; ok {
					usesBearer = true
					allowed = append(allowed, roles...)
				}
			}
			for role, token := range tokens {
				if usesBearer && slices.Contains(allowed, string(role)) {
					continue // o caso positivo é do teste do próprio issue
				}
				t.Run(name+" como "+string(role), func(t *testing.T) {
					r := call(t, srv, method, target, nil, bearer(token))
					if r.status != http.StatusForbidden && r.status != http.StatusUnauthorized {
						t.Errorf("queria 403, deu %d %v", r.status, r.body)
					}
					if usesBearer && r.status != http.StatusForbidden {
						t.Errorf("com sessão válida e papel errado é 403, deu %d", r.status)
					}
				})
			}
		}
	}

	// Garante que o teste não ficou vazio por engano (por exemplo, se a spec
	// deixasse de ter operações protegidas).
	if checked < 10 {
		t.Fatalf("só %d operações protegidas na spec: alguma coisa está errada", checked)
	}
	t.Logf("%d operações protegidas verificadas: %s", checked, strings.Repeat("·", min(checked, 40)))
}
