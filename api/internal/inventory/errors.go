package inventory

import "errors"

// ErrProductNotFound: o produto da entrada não existe.
var ErrProductNotFound = errors.New("o produto não existe")

// ErrUnitChanged: a unidade do produto mudou entre a validação da quantidade
// e a gravação do lote. A quantidade foi escrita na unidade antiga: não se
// grava (409), para a pessoa a confirmar.
var ErrUnitChanged = errors.New("a unidade do produto mudou entretanto")

// errProductKey: o Repo não conseguiu gravar o lote porque o par (produto,
// unidade) já não existe. O serviço decide se é ErrProductNotFound ou
// ErrUnitChanged.
var errProductKey = errors.New("o produto ou a unidade do lote já não existe")

// ValidationError é um dado inválido, com a mensagem para quem usa a app.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
