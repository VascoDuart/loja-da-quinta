package inventory

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// Service tem os casos de uso do stock. As escritas recebem quem as pede e
// voltam a verificar o papel, como nos outros módulos.
type Service struct {
	repo     Repo
	products Products
	now      func() time.Time
}

// NewService cria o serviço. now é o relógio (time.Now; um fixo nos testes).
func NewService(repo Repo, products Products, now func() time.Time) *Service {
	return &Service{repo: repo, products: products, now: now}
}

// RegisterLot regista uma entrada de stock (UC-04): valida o produto, a
// quantidade e as datas, e grava o lote com o movimento de entrada.
func (s *Service) RegisterLot(ctx context.Context, actor identity.Principal, in NewLot) (Lot, error) {
	if err := requireStaff(actor); err != nil {
		return Lot{}, err
	}
	in = in.Normalized()
	p, err := s.products.Product(ctx, in.ProductID)
	if err != nil {
		return Lot{}, err
	}
	if err := in.Validate(p, Day(s.now())); err != nil {
		return Lot{}, err
	}
	return s.repo.CreateLot(ctx, in, actor.UserID)
}

// StockMilli diz quanto stock vendável há de cada produto, em milésimos: só
// contam os lotes dentro da validade (um lote que expira hoje ainda conta).
// Os produtos sem stock não aparecem no mapa. Implementa a porta
// catalog.StockSource, por onde o catálogo público decide a disponibilidade.
func (s *Service) StockMilli(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	if len(productIDs) == 0 {
		return map[uuid.UUID]int64{}, nil
	}
	return s.repo.StockMilli(ctx, productIDs, Day(s.now()))
}

// requireStaff aceita o funcionário e o gestor (docs/security/authz-matrix.md).
func requireStaff(actor identity.Principal) error {
	if !actor.Role.Valid() || actor.UserID == uuid.Nil {
		return identity.ErrForbidden
	}
	return nil
}
