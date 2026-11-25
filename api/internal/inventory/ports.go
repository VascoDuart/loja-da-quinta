package inventory

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repo guarda os lotes e os movimentos de stock.
type Repo interface {
	// CreateLot grava o lote e o movimento de entrada (com quem o registou)
	// na mesma transação. ErrProductNotFound se o produto não existir.
	CreateLot(ctx context.Context, in NewLot, userID uuid.UUID) (Lot, error)
	// StockMilli devolve, para os produtos pedidos que têm stock, a soma do
	// que resta nos lotes com validade a partir de today. Os produtos sem
	// stock não aparecem.
	StockMilli(ctx context.Context, productIDs []uuid.UUID, today time.Time) (map[uuid.UUID]int64, error)
}

// Products é o que o stock precisa do catálogo, que é o dono dos produtos:
// este módulo nunca lê a tabela products. Na composição (internal/app) é o
// serviço do catálogo.
type Products interface {
	// Product devolve ErrProductNotFound se o produto não existir.
	Product(ctx context.Context, id uuid.UUID) (Product, error)
}
