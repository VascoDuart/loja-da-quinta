package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP descobre o IP do cliente.
//
// O X-Forwarded-For pode ser escrito por qualquer pessoa, por isso só se
// confia nas entradas acrescentadas pelos proxies conhecidos. Com
// trustedHops proxies à frente da API (Vercel e Render = 2), o IP real é a
// entrada nessa posição a contar do fim. Com trustedHops = 0, usa-se o
// endereço da ligação TCP.
func ClientIP(r *http.Request, trustedHops int) string {
	if trustedHops > 0 {
		var hops []string
		for _, v := range r.Header.Values("X-Forwarded-For") {
			for _, p := range strings.Split(v, ",") {
				if p = strings.TrimSpace(p); p != "" {
					hops = append(hops, p)
				}
			}
		}
		if i := len(hops) - trustedHops; i >= 0 && i < len(hops) {
			if ip := net.ParseIP(hops[i]); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
