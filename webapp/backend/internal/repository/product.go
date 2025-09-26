package repository

import (
	"backend/internal/model"
	"context"
)

type ProductRepository struct {
	db DBTX
}

func NewProductRepository(db DBTX) *ProductRepository {
	return &ProductRepository{db: db}
}

// 商品一覧を取得し、データベース側でページング処理を行う
func (r *ProductRepository) ListProducts(ctx context.Context, userID int, req model.ListRequest) ([]model.Product, int, error) {
	var products []model.Product
	var total int

	// まず総件数を取得
	countQuery := `SELECT COUNT(*) FROM products`
	args := []interface{}{}

	if req.Search != "" {
		countQuery += " WHERE (name LIKE ? OR description LIKE ?)"
		searchPattern := "%" + req.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	// ページング付きでデータを取得
	dataQuery := `
		SELECT product_id, name, value, weight, image, description
		FROM products
	`
	dataArgs := append([]interface{}{}, args...)

	if req.Search != "" {
		dataQuery += " WHERE (name LIKE ? OR description LIKE ?)"
	}

	dataQuery += " ORDER BY " + req.SortField + " " + req.SortOrder + " , product_id ASC"
	dataQuery += " LIMIT ? OFFSET ?"

	dataArgs = append(dataArgs, req.PageSize, req.Offset)

	err = r.db.SelectContext(ctx, &products, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}
