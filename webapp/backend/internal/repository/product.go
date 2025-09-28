package repository

import (
	"backend/internal/model"
	"context"
	"fmt"
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
        if req.Type == "prefix" {
            countQuery += " WHERE name >= ? AND name < ?"
            upper := req.Search + "\uffff"
            args = append(args, req.Search, upper)
        } else {
            // partial: FULLTEXT を活用し、fallback として LIKE
            countQuery += " WHERE MATCH(name, description) AGAINST (? IN NATURAL LANGUAGE MODE)"
            args = append(args, req.Search)
        }
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
	var dataArgs []interface{}

    if req.Search != "" {
        if req.Type == "prefix" {
            dataQuery += " WHERE name >= ? AND name < ?"
            upper := req.Search + "\uffff"
            dataArgs = append(dataArgs, req.Search, upper)
        } else {
            dataQuery += " WHERE MATCH(name, description) AGAINST (? IN NATURAL LANGUAGE MODE)"
            dataArgs = append(dataArgs, req.Search)
        }
    }

	// ソートの列をホワイトリストで制限
	sortField := sanitizeSortField(req.SortField, "product_id", map[string]string{
		"product_id": "product_id",
		"name":       "name",
		"value":      "value",
		"weight":     "weight",
		"image":      "image",
	})

	sortOrder := normalizeSortOrder(req.SortOrder, "ASC")

	dataQuery += fmt.Sprintf(" ORDER BY %s %s, product_id ASC", sortField, sortOrder)
	dataQuery += " LIMIT ? OFFSET ?"

	dataArgs = append(dataArgs, req.PageSize, req.Offset)

	err = r.db.SelectContext(ctx, &products, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}
