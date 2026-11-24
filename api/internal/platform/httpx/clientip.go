package httpx

import "context"

type clientIPKey struct{}

// WithClientIP devolve um contexto com o IP do cliente.
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// ClientIP devolve o IP do cliente guardado pelo middleware ClientIPContext,
// ou "" se não existir.
func ClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}
