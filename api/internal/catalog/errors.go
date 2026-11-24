package catalog

import "errors"

var (
	// ErrNotFound: a categoria ou o produto não existe.
	ErrNotFound = errors.New("não encontrado")
	// ErrNameTaken: já existe uma categoria ou um produto com este nome.
	ErrNameTaken = errors.New("nome repetido")
	// ErrInUse: a categoria tem produtos, ou o produto tem histórico (stock,
	// vendas). Não se apaga; um produto desativa-se.
	ErrInUse = errors.New("em uso")
	// ErrVersionMismatch: o produto mudou desde que o cliente o leu (412).
	ErrVersionMismatch = errors.New("o produto foi alterado entretanto")

	// ErrUnsupportedImage: não é JPEG, PNG nem WebP (verificado pelo conteúdo).
	ErrUnsupportedImage = errors.New("formato de imagem não suportado")
	// ErrImageTooLarge: mais de 5 MB ou de 40 megapíxeis.
	ErrImageTooLarge = errors.New("imagem demasiado grande")
	// ErrInvalidImage: diz ser uma imagem, mas não se consegue ler.
	ErrInvalidImage = errors.New("imagem inválida")
)

// ValidationError indica dados inválidos num campo. A mensagem é para
// mostrar a quem usa a app.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }
