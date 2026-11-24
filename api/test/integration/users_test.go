//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

func TestUserManagement(t *testing.T) {
	e := newEnv(t)
	maria := e.createUser(t, "maria@quinta.pt", identity.RoleGestor, pw)
	srv := e.server(t)
	gestor, _ := login(t, srv, "maria@quinta.pt")

	create := func(t *testing.T, email, role string) map[string]any {
		t.Helper()
		r := call(t, srv, "POST", "/users", map[string]string{"email": email, "name": "Pessoa", "role": role, "password": pw}, bearer(gestor))
		if r.status != http.StatusCreated {
			t.Fatalf("criar %s: %d %v", email, r.status, r.body)
		}
		return r.body
	}
	patch := func(t *testing.T, id any, body map[string]any) response {
		t.Helper()
		return call(t, srv, "PATCH", "/users/"+id.(string), body, bearer(gestor))
	}

	t.Run("o gestor cria e vê a lista, sem hashes", func(t *testing.T) {
		u := create(t, "Ana@Quinta.pt", "funcionario")
		if u["email"] != "ana@quinta.pt" || u["active"] != true || u["createdAt"] == nil {
			t.Errorf("utilizador criado: %v", u)
		}
		r := call(t, srv, "GET", "/users", nil, bearer(gestor))
		if r.status != http.StatusOK {
			t.Fatalf("listar: %d", r.status)
		}
		raw := call(t, srv, "GET", "/users", nil, bearer(gestor))
		if _, leaked := raw.body["passwordHash"]; leaked {
			t.Error("a lista nunca pode trazer o hash")
		}
	})

	t.Run("desativar termina as sessões e impede o login", func(t *testing.T) {
		u := create(t, "rui@quinta.pt", "funcionario")
		_, refresh := login(t, srv, "rui@quinta.pt")

		if r := patch(t, u["id"], map[string]any{"active": false}); r.status != http.StatusOK || r.body["active"] != false {
			t.Fatalf("desativar: %d %v", r.status, r.body)
		}
		if r := call(t, srv, "POST", "/auth/refresh", nil, withCookie(refresh)); r.status != http.StatusUnauthorized {
			t.Errorf("a sessão aberta devia ter terminado, refresh deu %d", r.status)
		}
		if r := call(t, srv, "POST", "/auth/login", map[string]string{"email": "rui@quinta.pt", "password": pw}, csrf); r.status != http.StatusUnauthorized {
			t.Errorf("conta desativada não entra, deu %d", r.status)
		}

		if r := patch(t, u["id"], map[string]any{"active": true}); r.status != http.StatusOK {
			t.Fatalf("reativar: %d", r.status)
		}
		login(t, srv, "rui@quinta.pt") // reativada, volta a entrar
	})

	t.Run("mudar o papel termina as sessões; no login seguinte vem o papel novo", func(t *testing.T) {
		u := create(t, "sara@quinta.pt", "funcionario")
		_, refresh := login(t, srv, "sara@quinta.pt")
		if r := patch(t, u["id"], map[string]any{"role": "gestor"}); r.status != http.StatusOK {
			t.Fatalf("promover: %d", r.status)
		}
		if r := call(t, srv, "POST", "/auth/refresh", nil, withCookie(refresh)); r.status != http.StatusUnauthorized {
			t.Errorf("a sessão com o papel antigo devia ter terminado, deu %d", r.status)
		}
		access, _ := login(t, srv, "sara@quinta.pt")
		if r := call(t, srv, "GET", "/users", nil, bearer(access)); r.status != http.StatusOK {
			t.Errorf("como gestora já devia ver os utilizadores, deu %d", r.status)
		}
	})

	t.Run("palavra-passe nova definida pelo gestor", func(t *testing.T) {
		u := create(t, "tiago@quinta.pt", "funcionario")
		if r := patch(t, u["id"], map[string]any{"password": "palavra-passe-nova-123"}); r.status != http.StatusOK {
			t.Fatalf("mudar a palavra-passe: %d", r.status)
		}
		if r := call(t, srv, "POST", "/auth/login", map[string]string{"email": "tiago@quinta.pt", "password": pw}, csrf); r.status != http.StatusUnauthorized {
			t.Errorf("a antiga não devia entrar, deu %d", r.status)
		}
		if r := call(t, srv, "POST", "/auth/login", map[string]string{"email": "tiago@quinta.pt", "password": "palavra-passe-nova-123"}, csrf); r.status != http.StatusOK {
			t.Errorf("a nova devia entrar, deu %d", r.status)
		}
	})

	t.Run("o gestor não se desativa nem se despromove", func(t *testing.T) {
		for _, body := range []map[string]any{{"active": false}, {"role": "funcionario"}} {
			if r := patch(t, maria.ID.String(), body); r.status != http.StatusConflict {
				t.Errorf("%v: queria 409, deu %d", body, r.status)
			}
		}
	})

	t.Run("erros: e-mail repetido, dados inválidos, utilizador inexistente", func(t *testing.T) {
		dup := call(t, srv, "POST", "/users", map[string]string{"email": "MARIA@quinta.pt", "name": "X", "role": "gestor", "password": pw}, bearer(gestor))
		if dup.status != http.StatusConflict {
			t.Errorf("e-mail repetido: queria 409, deu %d", dup.status)
		}
		blank := call(t, srv, "POST", "/users", map[string]string{"email": "x@quinta.pt", "name": "   ", "role": "gestor", "password": pw}, bearer(gestor))
		if blank.status != http.StatusUnprocessableEntity || blank.body["detail"] == nil {
			t.Errorf("nome em branco: queria 422 com detalhe, deu %d %v", blank.status, blank.body)
		}
		short := call(t, srv, "POST", "/users", map[string]string{"email": "y@quinta.pt", "name": "Y", "role": "gestor", "password": "curta"}, bearer(gestor))
		if short.status != http.StatusBadRequest {
			t.Errorf("palavra-passe curta (contrato): queria 400, deu %d", short.status)
		}
		admin := call(t, srv, "POST", "/users", map[string]string{"email": "z@quinta.pt", "name": "Z", "role": "admin", "password": pw}, bearer(gestor))
		if admin.status != http.StatusBadRequest {
			t.Errorf("papel inexistente (contrato): queria 400, deu %d", admin.status)
		}
		empty := patch(t, maria.ID.String(), map[string]any{})
		if empty.status != http.StatusBadRequest {
			t.Errorf("PATCH vazio: queria 400, deu %d", empty.status)
		}
		missing := patch(t, "00000000-0000-4000-8000-000000000000", map[string]any{"name": "X"})
		if missing.status != http.StatusNotFound {
			t.Errorf("inexistente: queria 404, deu %d", missing.status)
		}
	})
}
