package catalog

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repo guarda as categorias e os produtos.
type Repo interface {
	ListCategories(ctx context.Context) ([]Category, error)
	// CreateCategory e RenameCategory devolvem ErrNameTaken se o nome existir.
	CreateCategory(ctx context.Context, name string) (Category, error)
	RenameCategory(ctx context.Context, id uuid.UUID, name string) (Category, error)
	// DeleteCategory devolve ErrInUse se tiver produtos.
	DeleteCategory(ctx context.Context, id uuid.UUID) error

	ListProducts(ctx context.Context) ([]Product, error)
	GetProduct(ctx context.Context, id uuid.UUID) (Product, error)
	// CreateProduct devolve ErrNameTaken, ou um ValidationError se a
	// categoria não existir.
	CreateProduct(ctx context.Context, in ProductInput) (Product, error)
	// UpdateProduct só grava se o produto ainda estiver na versão
	// expectedVersion; senão devolve ErrVersionMismatch (ou ErrNotFound).
	UpdateProduct(ctx context.Context, id uuid.UUID, expectedVersion time.Time, in ProductInput) (Product, error)
	// DeleteProduct devolve ErrInUse se o produto tiver histórico.
	DeleteProduct(ctx context.Context, id uuid.UUID) error
}

// ImageStore guarda as imagens dos produtos (ADR-0014). Hoje é o
// PostgreSQL; se a loja crescer, troca-se por R2 ou S3 sem mudar o domínio.
type ImageStore interface {
	// Replace troca as duas versões numa transação. ErrNotFound se o
	// produto não existir.
	Replace(ctx context.Context, productID uuid.UUID, img ProcessedImage) error
	// Delete remove a imagem. ErrNotFound se o produto não existir.
	Delete(ctx context.Context, productID uuid.UUID) error
	// Get devolve uma versão ("thumb" ou "large") e a versão pública atual.
	Get(ctx context.Context, productID uuid.UUID, size string) (data []byte, sha256 []byte, version []byte, err error)
}
