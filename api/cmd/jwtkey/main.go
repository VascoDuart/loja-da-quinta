// Comando jwtkey: gera uma chave Ed25519 nova para os access tokens.
//
//	go run ./cmd/jwtkey
//
// O stdout tem só o valor de JWT_SIGNING_KEY ("kid:seed"), para se poder
// encaminhar diretamente para um gestor de segredos sem aparecer no ecrã.
// A chave pública, que vai para JWT_VERIFY_KEYS quando se trocar de chave,
// sai no stderr.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/VascoDuart/loja-da-quinta/api/internal/auth"
)

func main() {
	signing, public := auth.GenerateKey(time.Now())
	fmt.Fprintln(os.Stderr, "Chave pública (para JWT_VERIFY_KEYS depois de trocar de chave):", public)
	fmt.Print(signing)
}
