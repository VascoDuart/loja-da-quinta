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
	// O lote guarda unit, a unidade em que a quantidade foi validada: se a
	// do produto já não for essa (ou o produto já não existir), errProductKey.
	CreateLot(ctx context.Context, in NewLot, unit string, userID uuid.UUID) (Lot, error)
	// HasLots diz se o produto tem algum lote, com ou sem stock.
	HasLots(ctx context.Context, productID uuid.UUID) (bool, error)
	// Lot lê um lote, sem o bloquear. ErrLotNotFound se não existir.
	Lot(ctx context.Context, id uuid.UUID) (Lot, error)
	// ListLots devolve os lotes do produto que têm stock ou ainda estão
	// dentro da validade (a partir de today), pela ordem FEFO.
	ListLots(ctx context.Context, productID uuid.UUID, today time.Time) ([]Lot, error)
	// Move aplica um movimento a um lote numa só transação: bloqueia o lote
	// (SELECT ... FOR UPDATE), confirma com Lot.LeftAfter que não fica
	// negativo, atualiza o que resta e grava o movimento. Dois movimentos ao
	// mesmo lote ficam em fila: o segundo vê o que o primeiro deixou.
	// ErrLotNotFound, InsufficientStockError ou ValidationError.
	Move(ctx context.Context, m Movement) (Lot, RecordedMovement, error)
	// StockMilli devolve, para os produtos pedidos que têm stock, a soma do
	// que resta nos lotes com validade a partir de today. Os produtos sem
	// stock não aparecem.
	StockMilli(ctx context.Context, productIDs []uuid.UUID, today time.Time) (map[uuid.UUID]int64, error)
	// StockedLots devolve todos os lotes com stock (também os fora de
	// validade), por produto e pela ordem FEFO.
	StockedLots(ctx context.Context) ([]Lot, error)
}

// AlertDays regista os dias em que o alerta de stock saiu.
type AlertDays interface {
	// ClaimDay marca o dia; false se já estava marcado. Com dois pedidos ao
	// mesmo tempo, só um recebe true.
	ClaimDay(ctx context.Context, day time.Time) (bool, error)
	// ReleaseDay desmarca o dia (o envio falhou).
	ReleaseDay(ctx context.Context, day time.Time) error
}

// Email é um e-mail de texto simples.
type Email struct {
	To      []string
	Subject string
	Text    string
}

// Delivery diz por onde saiu um alerta.
type Delivery string

const (
	DeliveryEmail Delivery = "email"
	// DeliveryLog: sem serviço de e-mail configurado, ficou no log.
	DeliveryLog Delivery = "log"
)

// Notifier entrega um alerta (ResendNotifier ou LogNotifier).
type Notifier interface {
	Send(ctx context.Context, m Email) (Delivery, error)
}

// Recipients diz a quem vão os alertas. Na composição são os gestores ativos
// (módulo users) ou a lista de ALERTS_TO.
type Recipients interface {
	AlertRecipients(ctx context.Context) ([]string, error)
}

// Products é o que o stock precisa do catálogo, que é o dono dos produtos:
// este módulo nunca lê a tabela products. Na composição (internal/app) é o
// serviço do catálogo.
type Products interface {
	// Product devolve ErrProductNotFound se o produto não existir.
	Product(ctx context.Context, id uuid.UUID) (Product, error)
	// Products devolve todos os produtos, ativos ou não, por nome.
	Products(ctx context.Context) ([]Product, error)
}
