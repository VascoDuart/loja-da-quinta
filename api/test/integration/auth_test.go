//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

const pw = "palavra-passe-de-teste"

type response struct {
	status int
	header http.Header
	body   map[string]any
}

// call faz um pedido à API. headers pode ter "Cookie" e "Authorization".
func call(t *testing.T, srv *httptest.Server, method, path string, body any, headers map[string]string) response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, srv.URL+"/api/v1"+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return response{status: res.StatusCode, header: res.Header, body: out}
}

var csrf = map[string]string{"X-CSRF": "1"}

func withCookie(refresh string) map[string]string {
	return map[string]string{"X-CSRF": "1", "Cookie": "refresh_token=" + refresh}
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// refreshFrom lê o refresh token do Set-Cookie.
func refreshFrom(t *testing.T, r response) string {
	t.Helper()
	c, err := http.ParseSetCookie(r.header.Get("Set-Cookie"))
	if err != nil || c.Name != "refresh_token" {
		t.Fatalf("faltava o cookie refresh_token: %q", r.header.Get("Set-Cookie"))
	}
	return c.Value
}

func login(t *testing.T, srv *httptest.Server, email string) (access, refresh string) {
	t.Helper()
	r := call(t, srv, "POST", "/auth/login", map[string]string{"email": email, "password": pw}, csrf)
	if r.status != http.StatusOK {
		t.Fatalf("login de %s: %d %v", email, r.status, r.body)
	}
	return r.body["accessToken"].(string), refreshFrom(t, r)
}

func TestAuth(t *testing.T) {
	e := newEnv(t)
	gestor := e.createUser(t, "gestor@quinta.pt", identity.RoleGestor, pw)
	e.createUser(t, "funcionario@quinta.pt", identity.RoleFuncionario, pw)

	t.Run("fluxo completo: login, /me, renovação e logout", func(t *testing.T) {
		srv := e.server(t)
		r := call(t, srv, "POST", "/auth/login", map[string]string{"email": "Gestor@Quinta.pt", "password": pw}, csrf)
		if r.status != http.StatusOK {
			t.Fatalf("login: %d %v", r.status, r.body)
		}
		if r.body["tokenType"] != "Bearer" || r.body["expiresIn"] != float64(900) {
			t.Errorf("corpo inesperado: %v", r.body)
		}
		cookie := r.header.Get("Set-Cookie")
		for _, want := range []string{"HttpOnly", "Secure", "SameSite=Strict", "Path=/api/v1/auth", "Max-Age=604800"} {
			if !strings.Contains(cookie, want) {
				t.Errorf("o cookie devia ter %s: %s", want, cookie)
			}
		}
		if r.header.Get("Cache-Control") != "no-store" {
			t.Error("uma resposta com tokens não pode ficar em cache")
		}

		me := call(t, srv, "GET", "/me", nil, bearer(r.body["accessToken"].(string)))
		if me.status != http.StatusOK || me.body["email"] != "gestor@quinta.pt" || me.body["role"] != "gestor" || me.body["id"] != gestor.ID.String() {
			t.Fatalf("/me: %d %v", me.status, me.body)
		}

		refreshed := call(t, srv, "POST", "/auth/refresh", nil, withCookie(refreshFrom(t, r)))
		if refreshed.status != http.StatusOK || refreshed.body["accessToken"] == "" {
			t.Fatalf("refresh: %d %v", refreshed.status, refreshed.body)
		}
		next := refreshFrom(t, refreshed)
		if next == refreshFrom(t, r) {
			t.Error("o refresh token devia ter rodado")
		}

		out := call(t, srv, "POST", "/auth/logout", nil, withCookie(next))
		if out.status != http.StatusNoContent || !strings.Contains(out.header.Get("Set-Cookie"), "Max-Age=0") {
			t.Fatalf("logout: %d, Set-Cookie %q", out.status, out.header.Get("Set-Cookie"))
		}
		if again := call(t, srv, "POST", "/auth/refresh", nil, withCookie(next)); again.status != http.StatusUnauthorized {
			t.Errorf("depois do logout o refresh devia dar 401, deu %d", again.status)
		}
	})

	t.Run("reutilização de um refresh revoga a sessão inteira", func(t *testing.T) {
		srv := e.server(t)
		_, stolen := login(t, srv, "gestor@quinta.pt")
		legit := refreshFrom(t, call(t, srv, "POST", "/auth/refresh", nil, withCookie(stolen)))

		reuse := call(t, srv, "POST", "/auth/refresh", nil, withCookie(stolen))
		if reuse.status != http.StatusUnauthorized || !strings.Contains(reuse.header.Get("Set-Cookie"), "Max-Age=0") {
			t.Fatalf("reutilização: %d, Set-Cookie %q", reuse.status, reuse.header.Get("Set-Cookie"))
		}
		if r := call(t, srv, "POST", "/auth/refresh", nil, withCookie(legit)); r.status != http.StatusUnauthorized {
			t.Errorf("o token legítimo também devia ter sido revogado, deu %d", r.status)
		}
	})

	t.Run("dois refreshes ao mesmo tempo com o mesmo token: só um roda", func(t *testing.T) {
		srv := e.server(t)
		_, refresh := login(t, srv, "funcionario@quinta.pt")
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i := range codes {
			wg.Go(func() { codes[i] = call(t, srv, "POST", "/auth/refresh", nil, withCookie(refresh)).status })
		}
		wg.Wait()
		ok := 0
		for _, c := range codes {
			if c == http.StatusOK {
				ok++
			}
		}
		if ok != 1 {
			t.Errorf("exatamente um devia rodar o token (FOR UPDATE), códigos %v", codes)
		}
	})

	t.Run("refresh tokens guardam-se só como hash", func(t *testing.T) {
		srv := e.server(t)
		_, refresh := login(t, srv, "funcionario@quinta.pt")
		var found int
		if err := e.pool.QueryRow(context.Background(),
			"SELECT count(*) FROM refresh_tokens WHERE token_hash = $1 OR encode(token_hash, 'escape') = $2",
			[]byte(refresh), refresh).Scan(&found); err != nil {
			t.Fatal(err)
		}
		if found != 0 {
			t.Error("o refresh token em claro nunca pode estar na base de dados")
		}
	})

	t.Run("erro genérico: e-mail desconhecido e palavra-passe errada respondem igual", func(t *testing.T) {
		srv := e.server(t)
		wrong := call(t, srv, "POST", "/auth/login", map[string]string{"email": "gestor@quinta.pt", "password": "errada"}, csrf)
		unknown := call(t, srv, "POST", "/auth/login", map[string]string{"email": "ninguem@quinta.pt", "password": pw}, csrf)
		if wrong.status != http.StatusUnauthorized || unknown.status != http.StatusUnauthorized {
			t.Fatalf("códigos %d e %d, queria 401", wrong.status, unknown.status)
		}
		if wrong.body["detail"] != unknown.body["detail"] {
			t.Errorf("as mensagens deviam ser iguais: %q e %q", wrong.body["detail"], unknown.body["detail"])
		}
		if wrong.header.Get("Set-Cookie") != "" {
			t.Error("um login falhado não dá cookie")
		}
	})

	t.Run("5 falhas bloqueiam a conta: até a palavra-passe certa dá 429", func(t *testing.T) {
		srv := e.server(t)
		for range 5 {
			call(t, srv, "POST", "/auth/login", map[string]string{"email": "funcionario@quinta.pt", "password": "errada"}, csrf)
		}
		r := call(t, srv, "POST", "/auth/login", map[string]string{"email": "funcionario@quinta.pt", "password": pw}, csrf)
		if r.status != http.StatusTooManyRequests {
			t.Errorf("queria 429, deu %d %v", r.status, r.body)
		}
	})

	t.Run("sem X-CSRF os pedidos de sessão são recusados", func(t *testing.T) {
		srv := e.server(t)
		_, refresh := login(t, srv, "gestor@quinta.pt")
		cases := map[string]response{
			"login":   call(t, srv, "POST", "/auth/login", map[string]string{"email": "gestor@quinta.pt", "password": pw}, nil),
			"refresh": call(t, srv, "POST", "/auth/refresh", nil, map[string]string{"Cookie": "refresh_token=" + refresh}),
			"logout":  call(t, srv, "POST", "/auth/logout", nil, map[string]string{"Cookie": "refresh_token=" + refresh}),
		}
		for name, r := range cases {
			if r.status != http.StatusBadRequest {
				t.Errorf("%s sem X-CSRF: queria 400, deu %d", name, r.status)
			}
		}
		// O refresh não foi gasto pelo pedido recusado.
		if r := call(t, srv, "POST", "/auth/refresh", nil, withCookie(refresh)); r.status != http.StatusOK {
			t.Errorf("o refresh devia continuar válido, deu %d", r.status)
		}
	})

	t.Run("campos a mais no login são recusados", func(t *testing.T) {
		srv := e.server(t)
		r := call(t, srv, "POST", "/auth/login", map[string]string{"email": "gestor@quinta.pt", "password": pw, "role": "gestor"}, csrf)
		if r.status != http.StatusBadRequest {
			t.Errorf("queria 400, deu %d", r.status)
		}
	})
}

// TestAuthorizationMatrix testa docs/security/authz-matrix.md: cada linha
// com cada papel. As linhas dos endpoints que ainda não existem entram com
// os issues respetivos.
func TestAuthorizationMatrix(t *testing.T) {
	e := newEnv(t)
	e.createUser(t, "gestor@quinta.pt", identity.RoleGestor, pw)
	e.createUser(t, "funcionario@quinta.pt", identity.RoleFuncionario, pw)
	srv := e.server(t)

	target := e.createUser(t, "alvo@quinta.pt", identity.RoleFuncionario, pw)

	gestor, _ := login(t, srv, "gestor@quinta.pt")
	funcionario, _ := login(t, srv, "funcionario@quinta.pt")
	tokens := map[string]string{"visitante": "", "funcionario": funcionario, "gestor": gestor}

	newUser := map[string]string{"email": "novo@quinta.pt", "name": "Novo", "role": "funcionario", "password": pw}
	rows := []struct {
		method, path string
		body         any
		want         map[string]int
	}{
		{"GET", "/healthz", nil, map[string]int{"visitante": 200, "funcionario": 200, "gestor": 200}},
		{"GET", "/me", nil, map[string]int{"visitante": 401, "funcionario": 200, "gestor": 200}},
		{"GET", "/users", nil, map[string]int{"visitante": 401, "funcionario": 403, "gestor": 200}},
		{"POST", "/users", newUser, map[string]int{"visitante": 401, "funcionario": 403, "gestor": 201}},
		{"PATCH", "/users/" + target.ID.String(), map[string]string{"name": "Alvo"}, map[string]int{"visitante": 401, "funcionario": 403, "gestor": 200}},
	}
	for _, row := range rows {
		for who, token := range tokens {
			t.Run(row.method+" "+row.path+" como "+who, func(t *testing.T) {
				var h map[string]string
				if token != "" {
					h = bearer(token)
				}
				r := call(t, srv, row.method, row.path, row.body, h)
				if r.status != row.want[who] {
					t.Errorf("queria %d, deu %d %v", row.want[who], r.status, r.body)
				}
				if r.status == http.StatusUnauthorized && r.header.Get("WWW-Authenticate") == "" {
					t.Error("um 401 tem de trazer WWW-Authenticate")
				}
			})
		}
	}

	t.Run("token inválido dá 401", func(t *testing.T) {
		if r := call(t, srv, "GET", "/me", nil, bearer("nao.e.valido")); r.status != http.StatusUnauthorized {
			t.Errorf("queria 401, deu %d", r.status)
		}
	})
}
