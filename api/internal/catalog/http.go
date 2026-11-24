package catalog

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/gen"
	"github.com/VascoDuart/loja-da-quinta/api/internal/httpapi/problems"
	"github.com/VascoDuart/loja-da-quinta/api/internal/platform/identity"
)

// Endpoints implementa as operações da tag catalog da OpenAPI.
type Endpoints struct{ svc *Service }

// NewEndpoints cria os handlers.
func NewEndpoints(svc *Service) *Endpoints { return &Endpoints{svc: svc} }

const (
	msgForbidden        = "Só o gestor pode alterar o catálogo."
	msgCategoryNotFound = "Esta categoria não existe."
	msgProductNotFound  = "Este produto não existe."
	msgCategoryTaken    = "Já existe uma categoria com este nome."
	msgProductTaken     = "Já existe um produto com este nome."
	msgCategoryInUse    = "Esta categoria tem produtos. Muda-os de categoria antes de a apagar."
	msgProductInUse     = "Este produto já tem histórico (stock ou vendas) e não se pode apagar. Desativa-o."
	msgVersion          = "Outra pessoa alterou este produto entretanto. Recarrega para veres a versão atual."
	msgUnsupported      = "O ficheiro tem de ser uma imagem JPEG, PNG ou WebP."
	msgTooLarge         = "A imagem é demasiado grande: no máximo 5 MB e 40 megapíxeis."
	msgInvalidImage     = "Não foi possível ler a imagem. O ficheiro pode estar danificado."
	cacheForever        = "public, max-age=31536000, immutable"
	cacheCatalog        = "public, max-age=60"
)

// --- Categorias ------------------------------------------------------------

func (h *Endpoints) ListCategories(ctx context.Context, _ gen.ListCategoriesRequestObject) (gen.ListCategoriesResponseObject, error) {
	list, err := h.svc.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListCategories200JSONResponse, len(list))
	for i, c := range list {
		out[i] = toCategory(c)
	}
	return out, nil
}

func (h *Endpoints) CreateCategory(ctx context.Context, req gen.CreateCategoryRequestObject) (gen.CreateCategoryResponseObject, error) {
	c, err := h.svc.CreateCategory(ctx, actor(ctx), req.Body.Name)
	var ve *ValidationError
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.CreateCategory403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrNameTaken):
		return gen.CreateCategory409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, msgCategoryTaken)), nil
	case errors.As(err, &ve):
		return gen.CreateCategory422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	return gen.CreateCategory201JSONResponse(toCategory(c)), nil
}

func (h *Endpoints) RenameCategory(ctx context.Context, req gen.RenameCategoryRequestObject) (gen.RenameCategoryResponseObject, error) {
	c, err := h.svc.RenameCategory(ctx, actor(ctx), req.Id, req.Body.Name)
	var ve *ValidationError
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.RenameCategory403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrNotFound):
		return gen.RenameCategory404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, msgCategoryNotFound)), nil
	case errors.Is(err, ErrNameTaken):
		return gen.RenameCategory409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, msgCategoryTaken)), nil
	case errors.As(err, &ve):
		return gen.RenameCategory422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	return gen.RenameCategory200JSONResponse(toCategory(c)), nil
}

func (h *Endpoints) DeleteCategory(ctx context.Context, req gen.DeleteCategoryRequestObject) (gen.DeleteCategoryResponseObject, error) {
	err := h.svc.DeleteCategory(ctx, actor(ctx), req.Id)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.DeleteCategory403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrNotFound):
		return gen.DeleteCategory404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, msgCategoryNotFound)), nil
	case errors.Is(err, ErrInUse):
		return gen.DeleteCategory409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, msgCategoryInUse)), nil
	case err != nil:
		return nil, err
	}
	return gen.DeleteCategory204Response{}, nil
}

// --- Produtos --------------------------------------------------------------

func (h *Endpoints) ListProducts(ctx context.Context, _ gen.ListProductsRequestObject) (gen.ListProductsResponseObject, error) {
	list, err := h.svc.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListProducts200JSONResponse, len(list))
	for i, p := range list {
		out[i] = toProduct(p)
	}
	return out, nil
}

func (h *Endpoints) GetProduct(ctx context.Context, req gen.GetProductRequestObject) (gen.GetProductResponseObject, error) {
	p, err := h.svc.GetProduct(ctx, req.Id)
	if errors.Is(err, ErrNotFound) {
		return gen.GetProduct404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, msgProductNotFound)), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.GetProduct200JSONResponse{ProductWithETagJSONResponse: withETag(p)}, nil
}

func (h *Endpoints) CreateProduct(ctx context.Context, req gen.CreateProductRequestObject) (gen.CreateProductResponseObject, error) {
	p, err := h.svc.CreateProduct(ctx, actor(ctx), fromInput(*req.Body))
	var ve *ValidationError
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.CreateProduct403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrNameTaken):
		return gen.CreateProduct409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, msgProductTaken)), nil
	case errors.As(err, &ve):
		return gen.CreateProduct422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	return gen.CreateProduct201JSONResponse{ProductWithETagJSONResponse: withETag(p)}, nil
}

func (h *Endpoints) UpdateProduct(ctx context.Context, req gen.UpdateProductRequestObject) (gen.UpdateProductResponseObject, error) {
	p, err := h.svc.UpdateProduct(ctx, actor(ctx), req.Id, req.Params.IfMatch, fromInput(*req.Body))
	var ve *ValidationError
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.UpdateProduct403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrNotFound):
		return gen.UpdateProduct404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, msgProductNotFound)), nil
	case errors.Is(err, ErrVersionMismatch):
		return gen.UpdateProduct412ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusPreconditionFailed, msgVersion)), nil
	case errors.Is(err, ErrNameTaken):
		return gen.UpdateProduct409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, msgProductTaken)), nil
	case errors.As(err, &ve):
		return gen.UpdateProduct422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, ve.Message)), nil
	case err != nil:
		return nil, err
	}
	return gen.UpdateProduct200JSONResponse{ProductWithETagJSONResponse: withETag(p)}, nil
}

func (h *Endpoints) DeleteProduct(ctx context.Context, req gen.DeleteProductRequestObject) (gen.DeleteProductResponseObject, error) {
	err := h.svc.DeleteProduct(ctx, actor(ctx), req.Id)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.DeleteProduct403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrNotFound):
		return gen.DeleteProduct404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, msgProductNotFound)), nil
	case errors.Is(err, ErrInUse):
		return gen.DeleteProduct409ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusConflict, msgProductInUse)), nil
	case err != nil:
		return nil, err
	}
	return gen.DeleteProduct204Response{}, nil
}

// --- Imagens ---------------------------------------------------------------

func (h *Endpoints) PutProductImage(ctx context.Context, req gen.PutProductImageRequestObject) (gen.PutProductImageResponseObject, error) {
	// Lê no máximo um byte a mais do que o limite, para saber se passou.
	data, err := io.ReadAll(io.LimitReader(req.Body, MaxImageBytes+1))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) || len(data) > MaxImageBytes {
		return gen.PutProductImage413ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusRequestEntityTooLarge, msgTooLarge)), nil
	}
	if err != nil {
		return nil, err
	}

	version, err := h.svc.ReplaceImage(ctx, actor(ctx), req.Id, data)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.PutProductImage403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrNotFound):
		return gen.PutProductImage404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, msgProductNotFound)), nil
	case errors.Is(err, ErrUnsupportedImage):
		return gen.PutProductImage415ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnsupportedMediaType, msgUnsupported)), nil
	case errors.Is(err, ErrImageTooLarge):
		return gen.PutProductImage413ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusRequestEntityTooLarge, msgTooLarge)), nil
	case errors.Is(err, ErrInvalidImage):
		return gen.PutProductImage422ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusUnprocessableEntity, msgInvalidImage)), nil
	case err != nil:
		return nil, err
	}
	return gen.PutProductImage200JSONResponse{Version: version}, nil
}

func (h *Endpoints) DeleteProductImage(ctx context.Context, req gen.DeleteProductImageRequestObject) (gen.DeleteProductImageResponseObject, error) {
	err := h.svc.DeleteImage(ctx, actor(ctx), req.Id)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		return gen.DeleteProductImage403ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusForbidden, msgForbidden)), nil
	case errors.Is(err, ErrNotFound):
		return gen.DeleteProductImage404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, msgProductNotFound)), nil
	case err != nil:
		return nil, err
	}
	return gen.DeleteProductImage204Response{}, nil
}

func (h *Endpoints) GetProductImage(ctx context.Context, req gen.GetProductImageRequestObject) (gen.GetProductImageResponseObject, error) {
	data, etag, err := h.svc.PublicImage(ctx, req.Id, string(req.Params.Size), req.Params.V)
	if errors.Is(err, ErrNotFound) {
		return gen.GetProductImage404ApplicationProblemPlusJSONResponse(problems.New(ctx, http.StatusNotFound, "Esta imagem não existe.")), nil
	}
	if err != nil {
		return nil, err
	}
	return gen.GetProductImage200ImagejpegResponse{
		Body:          bytes.NewReader(data),
		ContentLength: int64(len(data)),
		Headers:       gen.GetProductImage200ResponseHeaders{CacheControl: cacheForever, ETag: etag},
	}, nil
}

// GetPublicCatalog devolve o catálogo público. O middleware de segurança põe
// no-store em tudo; aqui a resposta troca-o por 60 s de cache (o catálogo é
// o que mais tráfego recebe e muda pouco).
func (h *Endpoints) GetPublicCatalog(ctx context.Context, _ gen.GetPublicCatalogRequestObject) (gen.GetPublicCatalogResponseObject, error) {
	list, err := h.svc.PublicCatalog(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]gen.CatalogItem, len(list))
	for i, p := range list {
		items[i] = toCatalogItem(p)
	}
	return gen.GetPublicCatalog200JSONResponse{
		Body:    items,
		Headers: gen.GetPublicCatalog200ResponseHeaders{CacheControl: cacheCatalog},
	}, nil
}

// --- Conversões ------------------------------------------------------------

func toCatalogItem(p PublicProduct) gen.CatalogItem {
	out := gen.CatalogItem{
		Id: p.ID, Name: p.Name, CategoryName: p.CategoryName, Unit: gen.Unit(p.Unit),
		PriceCents: p.PriceCents, SeasonMonths: p.SeasonMonths,
		Availability: gen.Availability(p.Availability),
	}
	if p.ImageVersion != "" {
		out.Image = &gen.ProductImage{Version: p.ImageVersion}
	}
	return out
}

func actor(ctx context.Context) identity.Principal {
	p, _ := identity.From(ctx)
	return p
}

func toCategory(c Category) gen.Category {
	return gen.Category{Id: c.ID, Name: c.Name, ProductCount: c.ProductCount, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func fromInput(in gen.ProductInput) ProductInput {
	return ProductInput{
		CategoryID: in.CategoryId, Name: in.Name, Unit: Unit(in.Unit),
		PriceCents: in.PriceCents, VatRate: int(in.VatRate),
		SeasonMonths: in.SeasonMonths, LowStockMilli: in.LowStockMilli,
		IsPublic: in.IsPublic, Active: in.Active,
	}
}

func toProduct(p Product) gen.Product {
	out := gen.Product{
		Id: p.ID, CategoryId: p.CategoryID, CategoryName: p.CategoryName, Name: p.Name,
		Unit: gen.Unit(p.Unit), PriceCents: p.PriceCents, VatRate: gen.VatRate(p.VatRate),
		SeasonMonths: p.SeasonMonths, LowStockMilli: p.LowStockMilli,
		IsPublic: p.IsPublic, Active: p.Active,
		Version: p.Version(), CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	if p.ImageVersion != "" {
		out.Image = &gen.ProductImage{Version: p.ImageVersion}
	}
	return out
}

func withETag(p Product) gen.ProductWithETagJSONResponse {
	return gen.ProductWithETagJSONResponse{
		Body:    toProduct(p),
		Headers: gen.ProductWithETagResponseHeaders{ETag: `"` + p.Version() + `"`},
	}
}
