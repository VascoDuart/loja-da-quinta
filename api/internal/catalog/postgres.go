package catalog

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/db/sqlcgen"
)

// PostgresStore implementa Repo e ImageStore com as queries do sqlc
// (queries/catalog.sql).
type PostgresStore struct{ pool *pgxpool.Pool }

// NewPostgresStore cria o store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

var (
	_ Repo       = (*PostgresStore)(nil)
	_ ImageStore = (*PostgresStore)(nil)
)

func (s *PostgresStore) q() *sqlcgen.Queries { return sqlcgen.New(s.pool) }

// --- Categorias ------------------------------------------------------------

func (s *PostgresStore) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.q().ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar as categorias: %w", err)
	}
	out := make([]Category, len(rows))
	for i, r := range rows {
		out[i] = Category{ID: r.ID, Name: r.Name, ProductCount: int(r.ProductCount), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}

func (s *PostgresStore) CreateCategory(ctx context.Context, name string) (Category, error) {
	r, err := s.q().CreateCategory(ctx, name)
	if err != nil {
		return Category{}, mapErr(err, "criar a categoria", nil)
	}
	return Category{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}, nil
}

func (s *PostgresStore) RenameCategory(ctx context.Context, id uuid.UUID, name string) (Category, error) {
	r, err := s.q().RenameCategory(ctx, sqlcgen.RenameCategoryParams{ID: id, Name: name})
	if err != nil {
		return Category{}, mapErr(err, "mudar o nome da categoria", nil)
	}
	// A contagem de produtos não muda ao mudar o nome; lê-se da lista.
	return s.categoryWithCount(ctx, Category{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
}

func (s *PostgresStore) categoryWithCount(ctx context.Context, c Category) (Category, error) {
	all, err := s.ListCategories(ctx)
	if err != nil {
		return Category{}, err
	}
	for _, x := range all {
		if x.ID == c.ID {
			return x, nil
		}
	}
	return c, nil
}

func (s *PostgresStore) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	n, err := s.q().DeleteCategory(ctx, id)
	if err != nil {
		return mapErr(err, "apagar a categoria", ErrInUse)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Produtos --------------------------------------------------------------

func (s *PostgresStore) ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := s.q().ListProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar os produtos: %w", err)
	}
	out := make([]Product, len(rows))
	for i, r := range rows {
		out[i] = productFromRow(sqlcgen.GetProductRow(r))
	}
	return out, nil
}

func (s *PostgresStore) GetProduct(ctx context.Context, id uuid.UUID) (Product, error) {
	r, err := s.q().GetProduct(ctx, id)
	if err != nil {
		return Product{}, mapErr(err, "ler o produto", nil)
	}
	return productFromRow(r), nil
}

func (s *PostgresStore) CreateProduct(ctx context.Context, in ProductInput) (Product, error) {
	id, err := s.q().CreateProduct(ctx, sqlcgen.CreateProductParams{
		CategoryID: in.CategoryID, Name: in.Name, Unit: string(in.Unit),
		PriceCents: int32(in.PriceCents), VatRate: int16(in.VatRate), //nolint:gosec // G115: validados antes
		SeasonMonths: months(in.SeasonMonths), LowStockMilli: in.LowStockMilli,
		IsPublic: in.IsPublic, Active: in.Active,
	})
	if err != nil {
		return Product{}, mapErr(err, "criar o produto", errNoCategory)
	}
	return s.GetProduct(ctx, id)
}

func (s *PostgresStore) UpdateProduct(ctx context.Context, id uuid.UUID, expected time.Time, in ProductInput) (Product, error) {
	_, err := s.q().UpdateProduct(ctx, sqlcgen.UpdateProductParams{
		ID: id, ExpectedUpdatedAt: expected,
		CategoryID: in.CategoryID, Name: in.Name, Unit: string(in.Unit),
		PriceCents: int32(in.PriceCents), VatRate: int16(in.VatRate), //nolint:gosec // G115: validados antes
		SeasonMonths: months(in.SeasonMonths), LowStockMilli: in.LowStockMilli,
		IsPublic: in.IsPublic, Active: in.Active,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Nenhuma linha: ou o produto não existe, ou mudou entretanto.
		exists, err := s.q().ProductExists(ctx, id)
		if err != nil {
			return Product{}, fmt.Errorf("ler o produto: %w", err)
		}
		if !exists {
			return Product{}, ErrNotFound
		}
		return Product{}, ErrVersionMismatch
	}
	if err != nil {
		return Product{}, mapErr(err, "gravar o produto", errNoCategory)
	}
	return s.GetProduct(ctx, id)
}

func (s *PostgresStore) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	n, err := s.q().DeleteProduct(ctx, id)
	if err != nil {
		return mapErr(err, "apagar o produto", ErrInUse)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListPublicProducts(ctx context.Context) ([]Product, error) {
	rows, err := s.q().ListPublicProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar o catálogo público: %w", err)
	}
	out := make([]Product, len(rows))
	for i, r := range rows {
		p := Product{
			ProductInput: ProductInput{
				Name: r.Name, Unit: Unit(r.Unit), PriceCents: int(r.PriceCents),
				SeasonMonths: make([]int, len(r.SeasonMonths)), LowStockMilli: r.LowStockMilli,
				IsPublic: true, Active: true,
			},
			ID: r.ID, CategoryName: r.CategoryName,
		}
		for j, m := range r.SeasonMonths {
			p.SeasonMonths[j] = int(m)
		}
		if len(r.ImageSha256) > 0 {
			p.ImageVersion = hex.EncodeToString(r.ImageSha256)
		}
		out[i] = p
	}
	return out, nil
}

// --- Imagens ---------------------------------------------------------------

func (s *PostgresStore) Replace(ctx context.Context, productID uuid.UUID, img ProcessedImage) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar a transação: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // não faz nada depois do Commit
	q := sqlcgen.New(tx)

	if _, err := q.LockProduct(ctx, productID); err != nil {
		return mapErr(err, "ler o produto", nil)
	}
	if _, err := q.DeleteProductImages(ctx, productID); err != nil {
		return fmt.Errorf("apagar a imagem antiga: %w", err)
	}
	for size, v := range map[string]EncodedImage{"large": img.Large, "thumb": img.Thumb} {
		if err := q.InsertProductImage(ctx, sqlcgen.InsertProductImageParams{
			ProductID: productID, Size: size, Data: v.Data,
			Width: int32(v.Width), Height: int32(v.Height), Sha256: v.SHA256[:], //nolint:gosec // G115: no máximo 800
		}); err != nil {
			return fmt.Errorf("gravar a imagem: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("gravar a imagem: %w", err)
	}
	return nil
}

func (s *PostgresStore) Delete(ctx context.Context, productID uuid.UUID) error {
	exists, err := s.q().ProductExists(ctx, productID)
	if err != nil {
		return fmt.Errorf("ler o produto: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	_, err = s.q().DeleteProductImages(ctx, productID)
	return err
}

func (s *PostgresStore) Get(ctx context.Context, productID uuid.UUID, size string) ([]byte, []byte, []byte, error) {
	r, err := s.q().GetProductImage(ctx, sqlcgen.GetProductImageParams{ProductID: productID, Size: size})
	if err != nil {
		return nil, nil, nil, mapErr(err, "ler a imagem", nil)
	}
	return r.Data, r.Sha256, r.Version, nil
}

// --- Conversões ------------------------------------------------------------

func productFromRow(r sqlcgen.GetProductRow) Product {
	p := Product{
		ProductInput: ProductInput{
			CategoryID: r.CategoryID, Name: r.Name, Unit: Unit(r.Unit),
			PriceCents: int(r.PriceCents), VatRate: int(r.VatRate),
			SeasonMonths: make([]int, len(r.SeasonMonths)), LowStockMilli: r.LowStockMilli,
			IsPublic: r.IsPublic, Active: r.Active,
		},
		ID: r.ID, CategoryName: r.CategoryName, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	for i, m := range r.SeasonMonths {
		p.SeasonMonths[i] = int(m)
	}
	if len(r.ImageSha256) > 0 {
		p.ImageVersion = hex.EncodeToString(r.ImageSha256)
	}
	return p
}

func months(ms []int) []int16 {
	out := make([]int16, len(ms))
	for i, m := range ms {
		out[i] = int16(m) //nolint:gosec // G115: de 1 a 12, validado antes
	}
	return out
}

// mapErr traduz os erros do PostgreSQL em erros do domínio. onFK é o que uma
// violação de chave estrangeira quer dizer nesta operação: ao gravar um
// produto, a categoria não existe; ao apagar, há algo que depende do registo.
func mapErr(err error, what string, onFK error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation: nome repetido
			return ErrNameTaken
		case "23503": // foreign_key_violation
			if onFK != nil {
				return onFK
			}
		}
	}
	return fmt.Errorf("%s: %w", what, err)
}

// errNoCategory é o erro de gravar um produto numa categoria que não existe.
var errNoCategory = &ValidationError{Field: "categoryId", Message: "A categoria escolhida não existe."}
