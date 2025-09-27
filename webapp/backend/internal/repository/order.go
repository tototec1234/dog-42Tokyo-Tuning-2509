package repository

import (
	"backend/internal/model"
	"context"
	"database/sql"
	"fmt"
    "strings"

	"github.com/jmoiron/sqlx"
)

type OrderRepository struct {
	db DBTX
}

func NewOrderRepository(db DBTX) *OrderRepository {
	return &OrderRepository{db: db}
}

// 注文を作成し、生成された注文IDを返す
func (r *OrderRepository) Create(ctx context.Context, order *model.Order) (string, error) {
	query := `INSERT INTO orders (user_id, product_id, shipped_status, created_at) VALUES (?, ?, 'shipping', NOW())`
	result, err := r.db.ExecContext(ctx, query, order.UserID, order.ProductID)
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

// 複数レコードを一括挿入し、生成された order_id の配列を返す
func (r *OrderRepository) CreateBulk(ctx context.Context, userID int, productID int, quantity int) ([]int64, error) {
    if quantity <= 0 {
        return nil, nil
    }

    // VALUES プレースホルダーを構築
    // (user_id, product_id, 'shipping', NOW()) を quantity 回
    valuesPlaceholders := make([]string, 0, quantity)
    args := make([]interface{}, 0, quantity*2)
    for i := 0; i < quantity; i++ {
        valuesPlaceholders = append(valuesPlaceholders, "(?, ?, 'shipping', NOW())")
        args = append(args, userID, productID)
    }

    query := "INSERT INTO orders (user_id, product_id, shipped_status, created_at) VALUES " +
        sqlx.Rebind(sqlx.QUESTION, fmt.Sprintf("%s", strings.Join(valuesPlaceholders, ",")))

    // 注意: mysql の LastInsertId は最初の ID を返す。RowsAffected から連番を算出
    result, err := r.db.ExecContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    firstID, err := result.LastInsertId()
    if err != nil {
        return nil, err
    }
    rows, err := result.RowsAffected()
    if err != nil {
        return nil, err
    }

    ids := make([]int64, 0, rows)
    for i := int64(0); i < rows; i++ {
        ids = append(ids, firstID+i)
    }
    return ids, nil
}

// 複数の注文IDのステータスを一括で更新
// 主に配送ロボットが注文を引き受けた際に一括更新をするために使用
func (r *OrderRepository) UpdateStatuses(ctx context.Context, orderIDs []int64, newStatus string) error {
	if len(orderIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In("UPDATE orders SET shipped_status = ? WHERE order_id IN (?)", newStatus, orderIDs)
	if err != nil {
		return err
	}
	query = r.db.Rebind(query)
	_, err = r.db.ExecContext(ctx, query, args...)
	return err
}

// 配送中(shipped_status:shipping)の注文一覧を取得
func (r *OrderRepository) GetShippingOrders(ctx context.Context) ([]model.Order, error) {
	var orders []model.Order
	query := `
        SELECT
            o.order_id,
            p.weight,
            p.value
        FROM orders o
        JOIN products p ON o.product_id = p.product_id
        WHERE o.shipped_status = 'shipping'
    `
	err := r.db.SelectContext(ctx, &orders, query)
	return orders, err
}

// 注文履歴一覧を取得（JOINを使用してN+1問題を解決）
func (r *OrderRepository) ListOrders(ctx context.Context, userID int, req model.ListRequest) ([]model.Order, int, error) {
	var total int

    // 総件数を取得（検索なしの場合は JOIN 回避）
    countQuery := `
        SELECT COUNT(*)
        FROM orders o
        WHERE o.user_id = ?
    `
    args := []interface{}{userID}

    if req.Search != "" {
        if req.Type == "prefix" {
            countQuery += " AND EXISTS (SELECT 1 FROM products p WHERE p.product_id = o.product_id AND p.name >= ? AND p.name < ?)"
            upper := req.Search + "\uffff"
            args = append(args, req.Search, upper)
        } else {
            // partial は FULLTEXT 優先
            countQuery += " AND EXISTS (SELECT 1 FROM products p WHERE p.product_id = o.product_id AND MATCH(p.name, p.description) AGAINST (? IN NATURAL LANGUAGE MODE))"
            args = append(args, req.Search)
        }
    }

	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	// ページング付きでデータを取得
	dataQuery := `
        SELECT o.order_id, o.product_id, p.name as product_name, o.shipped_status, o.created_at, o.arrived_at
        FROM orders o
        JOIN products p ON o.product_id = p.product_id
        WHERE o.user_id = ?
    `
	dataArgs := []interface{}{userID}

    if req.Search != "" {
        if req.Type == "prefix" {
            dataQuery += " AND p.name >= ? AND p.name < ?"
            upper := req.Search + "\uffff"
            dataArgs = append(dataArgs, req.Search, upper)
        } else {
            dataQuery += " AND MATCH(p.name, p.description) AGAINST (? IN NATURAL LANGUAGE MODE)"
            dataArgs = append(dataArgs, req.Search)
        }
    }

	// ソート条件を構築
	var orderBy string
    // ソート順のホワイトリスト化
    allowedOrder := req.SortOrder
    if allowedOrder != "ASC" && allowedOrder != "DESC" {
        allowedOrder = "ASC"
    }
    switch req.SortField {
    case "product_name":
        orderBy = "p.name " + allowedOrder
    case "created_at":
        orderBy = "o.created_at " + allowedOrder
    case "shipped_status":
        orderBy = "o.shipped_status " + allowedOrder
    case "arrived_at":
        orderBy = "o.arrived_at " + allowedOrder
    case "order_id":
        fallthrough
    default:
        orderBy = "o.order_id " + allowedOrder
    }

	dataQuery += " ORDER BY " + orderBy + ", o.order_id ASC"
	dataQuery += " LIMIT ? OFFSET ?"
	dataArgs = append(dataArgs, req.PageSize, req.Offset)

	type orderRow struct {
		OrderID       int          `db:"order_id"`
		ProductID     int          `db:"product_id"`
		ProductName   string       `db:"product_name"`
		ShippedStatus string       `db:"shipped_status"`
		CreatedAt     sql.NullTime `db:"created_at"`
		ArrivedAt     sql.NullTime `db:"arrived_at"`
	}

	var ordersRaw []orderRow
	err = r.db.SelectContext(ctx, &ordersRaw, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, err
	}

	var orders []model.Order
	for _, o := range ordersRaw {
		orders = append(orders, model.Order{
			OrderID:       int64(o.OrderID),
			ProductID:     o.ProductID,
			ProductName:   o.ProductName,
			ShippedStatus: o.ShippedStatus,
			CreatedAt:     o.CreatedAt.Time,
			ArrivedAt:     o.ArrivedAt,
		})
	}

	return orders, total, nil
}
