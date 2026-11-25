package inventory

import "errors"

// ErrProductNotFound: o produto da entrada não existe.
var ErrProductNotFound = errors.New("o produto não existe")

// ValidationError é um dado inválido, com a mensagem para quem usa a app.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
