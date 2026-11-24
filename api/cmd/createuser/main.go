// Comando createuser: cria um utilizador interno diretamente na base de
// dados. Serve para criar o primeiro gestor e as contas dos testes E2E; a
// gestão normal de utilizadores é feita pelo gestor na app (#9).
//
//	DATABASE_URL=... go run ./cmd/createuser -email maria@quinta.pt -name Maria -role gestor
//
// A palavra-passe nunca vai na linha de comandos (ficaria no histórico):
// pede-se no terminal sem eco, lê-se de NEW_USER_PASSWORD ou da primeira
// linha do stdin.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/password"
	"github.com/VascoDuart/loja-da-quinta/api/internal/users"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run() error {
	email := flag.String("email", "", "e-mail do utilizador (obrigatório)")
	name := flag.String("name", "", "nome a mostrar (obrigatório)")
	role := flag.String("role", "", "gestor ou funcionario (obrigatório)")
	ifMissing := flag.Bool("if-missing", false, "não falha se o e-mail já existir (para scripts)")
	flag.Parse()

	if *email == "" || *name == "" || *role == "" {
		flag.Usage()
		return errors.New("-email, -name e -role são obrigatórios")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL é obrigatória")
	}
	pw, err := readPassword()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	pool, err := db.NewPool(ctx, url, 1)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Sem SessionRevoker: este comando só cria utilizadores.
	svc, err := users.NewService(ctx, users.NewPostgresRepo(pool), password.New(password.OWASP, 1), nil)
	if err != nil {
		return err
	}
	u, err := svc.Create(ctx, users.NewUser{Email: *email, Name: *name, Role: identity.Role(*role), Password: pw})
	if errors.Is(err, users.ErrEmailTaken) && *ifMissing {
		fmt.Println("O utilizador já existe; nada a fazer.")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("Utilizador criado: %s (%s)\n", u.Email, u.Role)
	return nil
}

func readPassword() (string, error) {
	if pw, ok := os.LookupEnv("NEW_USER_PASSWORD"); ok {
		return pw, nil
	}
	fd := int(os.Stdin.Fd()) //nolint:gosec // G115: um descritor de ficheiro cabe num int
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("não foi possível ler a palavra-passe do stdin")
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	fmt.Fprint(os.Stderr, "Palavra-passe (12 a 128 caracteres): ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Repete a palavra-passe: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("as palavras-passe não coincidem")
	}
	return string(first), nil
}
