package catalog

import (
	"context"
	"crypto/subtle"
	"encoding/hex"

	"github.com/google/uuid"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// Service tem os casos de uso do catálogo. As escritas recebem quem as pede
// e voltam a verificar o papel, como no módulo users.
type Service struct {
	repo   Repo
	images ImageStore
	// Uma imagem de 40 MP descodificada ocupa centenas de MB: processa-se uma
	// de cada vez, para caber nos 512 MB da Render.
	imageSlots chan struct{}
	stock      StockSource
}

// NewService cria o serviço.
func NewService(repo Repo, images ImageStore) *Service {
	return &Service{repo: repo, images: images, imageSlots: make(chan struct{}, 1), stock: UntrackedStock{}}
}

// WithStock liga o stock real ao catálogo público (#12). Sem isto, todos os
// produtos aparecem como disponíveis.
func (s *Service) WithStock(stock StockSource) *Service {
	s.stock = stock
	return s
}

// --- Categorias ------------------------------------------------------------

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *Service) CreateCategory(ctx context.Context, actor identity.Principal, name string) (Category, error) {
	if err := requireGestor(actor); err != nil {
		return Category{}, err
	}
	name = NormalizeName(name)
	if err := ValidateCategoryName(name); err != nil {
		return Category{}, err
	}
	return s.repo.CreateCategory(ctx, name)
}

func (s *Service) RenameCategory(ctx context.Context, actor identity.Principal, id uuid.UUID, name string) (Category, error) {
	if err := requireGestor(actor); err != nil {
		return Category{}, err
	}
	name = NormalizeName(name)
	if err := ValidateCategoryName(name); err != nil {
		return Category{}, err
	}
	return s.repo.RenameCategory(ctx, id, name)
}

func (s *Service) DeleteCategory(ctx context.Context, actor identity.Principal, id uuid.UUID) error {
	if err := requireGestor(actor); err != nil {
		return err
	}
	return s.repo.DeleteCategory(ctx, id)
}

// --- Produtos --------------------------------------------------------------

func (s *Service) ListProducts(ctx context.Context) ([]Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *Service) GetProduct(ctx context.Context, id uuid.UUID) (Product, error) {
	return s.repo.GetProduct(ctx, id)
}

func (s *Service) CreateProduct(ctx context.Context, actor identity.Principal, in ProductInput) (Product, error) {
	if err := requireGestor(actor); err != nil {
		return Product{}, err
	}
	in = in.Normalized()
	if err := in.Validate(); err != nil {
		return Product{}, err
	}
	return s.repo.CreateProduct(ctx, in)
}

// UpdateProduct substitui os dados de um produto, se ainda estiver na versão
// que o gestor leu (If-Match). Senão, ErrVersionMismatch.
func (s *Service) UpdateProduct(ctx context.Context, actor identity.Principal, id uuid.UUID, ifMatch string, in ProductInput) (Product, error) {
	if err := requireGestor(actor); err != nil {
		return Product{}, err
	}
	expected, ok := ParseVersion(ifMatch)
	if !ok {
		return Product{}, ErrVersionMismatch
	}
	in = in.Normalized()
	if err := in.Validate(); err != nil {
		return Product{}, err
	}
	return s.repo.UpdateProduct(ctx, id, expected, in)
}

func (s *Service) DeleteProduct(ctx context.Context, actor identity.Principal, id uuid.UUID) error {
	if err := requireGestor(actor); err != nil {
		return err
	}
	return s.repo.DeleteProduct(ctx, id)
}

// PublicCatalog devolve os produtos ativos e públicos, com a disponibilidade
// calculada a partir do stock. É público: não recebe quem pede.
func (s *Service) PublicCatalog(ctx context.Context) ([]PublicProduct, error) {
	products, err := s.repo.ListPublicProducts(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(products))
	for i, p := range products {
		ids[i] = p.ID
	}
	stock, err := s.stock.StockMilli(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]PublicProduct, len(products))
	for i, p := range products {
		availability := Available
		if milli, tracked := stock[p.ID]; tracked {
			availability = AvailabilityOf(milli, p.LowStockMilli)
		}
		out[i] = PublicProduct{
			ID: p.ID, Name: p.Name, CategoryName: p.CategoryName, Unit: p.Unit,
			PriceCents: p.PriceCents, SeasonMonths: p.SeasonMonths,
			ImageVersion: p.ImageVersion, Availability: availability,
		}
	}
	return out, nil
}

// --- Imagens ---------------------------------------------------------------

// ReplaceImage valida, reprocessa e grava a imagem de um produto. Devolve a
// versão nova (o hash, que entra no URL público).
func (s *Service) ReplaceImage(ctx context.Context, actor identity.Principal, productID uuid.UUID, data []byte) (string, error) {
	if err := requireGestor(actor); err != nil {
		return "", err
	}
	select {
	case s.imageSlots <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	img, err := ProcessImage(data)
	<-s.imageSlots
	if err != nil {
		return "", err
	}
	if err := s.images.Replace(ctx, productID, img); err != nil {
		return "", err
	}
	return hex.EncodeToString(img.Large.SHA256[:]), nil
}

func (s *Service) DeleteImage(ctx context.Context, actor identity.Principal, productID uuid.UUID) error {
	if err := requireGestor(actor); err != nil {
		return err
	}
	return s.images.Delete(ctx, productID)
}

// PublicImage devolve uma versão da imagem, só se version for a atual. Assim
// o URL é imutável (pode ficar em cache para sempre) e as imagens não se
// adivinham pelo ID do produto.
func (s *Service) PublicImage(ctx context.Context, productID uuid.UUID, size, version string) (data []byte, etag string, err error) {
	want, err := hex.DecodeString(version)
	if err != nil || len(want) != 32 {
		return nil, "", ErrNotFound
	}
	data, sum, current, err := s.images.Get(ctx, productID, size)
	if err != nil {
		return nil, "", err
	}
	if subtle.ConstantTimeCompare(current, want) != 1 {
		return nil, "", ErrNotFound
	}
	return data, `"` + hex.EncodeToString(sum) + `"`, nil
}

func requireGestor(actor identity.Principal) error {
	if actor.Role != identity.RoleGestor {
		return identity.ErrForbidden
	}
	return nil
}
