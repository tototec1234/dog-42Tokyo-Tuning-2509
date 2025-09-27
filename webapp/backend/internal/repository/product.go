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
        // 前綴検索を範囲検索で最適化可能だが、ここでは仕様維持のため LIKE を使用
        // プレフィックス検索は name >= prefix AND name < prefix_max に置換可能
        if req.Type == "prefix" {
            // 範囲検索へ置換（データ内容は不変）
            countQuery += " WHERE name >= ? AND name < ?"
            // 最大値生成：prefix + 最高の補足文字 (U+FFFF) ではなく、次の文字境界を用いる
            // ここでは簡易に prefix + "\uffff" を上界として使用
            upper := req.Search + "\uffff"
            args = append(args, req.Search, upper)
        } else {
            countQuery += " WHERE (name LIKE ? OR description LIKE ?)"
            searchPattern := "%" + req.Search + "%"
            args = append(args, searchPattern, searchPattern)
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
            dataQuery += " WHERE (name LIKE ? OR description LIKE ?)"
            searchPattern := "%" + req.Search + "%"
            dataArgs = append(dataArgs, searchPattern, searchPattern)
        }
    }

    // ソートの列をホワイトリストで制限
    sortField := req.SortField
    switch sortField {
    case "product_id", "name", "value", "weight", "image":
        // OK
    default:
        sortField = "product_id"
    }

    sortOrder := req.SortOrder
    if sortOrder != "ASC" && sortOrder != "DESC" {
        sortOrder = "ASC"
    }

    dataQuery += " ORDER BY " + sortField + " " + sortOrder + ", product_id ASC"
    dataQuery += " LIMIT ? OFFSET ?"

    dataArgs = append(dataArgs, req.PageSize, req.Offset)

	err = r.db.SelectContext(ctx, &products, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}
