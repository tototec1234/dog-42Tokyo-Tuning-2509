package repository

import (
	"backend/internal/model"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// CacheEntry 缓存条目结构
type CacheEntry struct {
	Products  []model.Product
	Total     int
	ExpiresAt time.Time
}

// ProductRepository 产品仓库，包含数据库连接和内存缓存
type ProductRepository struct {
	db    DBTX
	cache map[string]*CacheEntry
	mutex sync.RWMutex
	ttl   time.Duration
}

func NewProductRepository(db DBTX) *ProductRepository {
	return &ProductRepository{
		db:    db,
		cache: make(map[string]*CacheEntry),
		ttl:   5 * time.Minute, // 缓存5分钟
	}
}

// generateCacheKey 根据请求参数生成缓存键，只包含影响数据库查询结果的参数
func (r *ProductRepository) generateCacheKey(userID int, req model.ListRequest) string {
	// 只包含真正影响查询结果的参数
	keyData := fmt.Sprintf("search:%s|type:%s|page:%d|size:%d|sort:%s|order:%s|offset:%d",
		req.Search, req.Type, req.Page, req.PageSize, req.SortField, req.SortOrder, req.Offset)
	
	// 使用MD5生成短的缓存键
	hash := md5.Sum([]byte(keyData))
	return hex.EncodeToString(hash[:])
}

// getFromCache 从缓存中获取数据
func (r *ProductRepository) getFromCache(key string) ([]model.Product, int, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	entry, exists := r.cache[key]
	if !exists {
		return nil, 0, false
	}
	
	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		// 过期了，但不在这里删除，避免在读锁中写操作
		return nil, 0, false
	}
	
	return entry.Products, entry.Total, true
}

// setToCache 将数据存入缓存
func (r *ProductRepository) setToCache(key string, products []model.Product, total int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	// 清理过期的缓存条目
	now := time.Now()
	for k, v := range r.cache {
		if now.After(v.ExpiresAt) {
			delete(r.cache, k)
		}
	}
	
	// 添加新的缓存条目
	r.cache[key] = &CacheEntry{
		Products:  products,
		Total:     total,
		ExpiresAt: now.Add(r.ttl),
	}
}

// ClearCache 清空所有缓存（可用于数据更新后）
func (r *ProductRepository) ClearCache() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.cache = make(map[string]*CacheEntry)
}

// 商品一覧を取得し、データベース側でページング処理を行う
func (r *ProductRepository) ListProducts(ctx context.Context, userID int, req model.ListRequest) ([]model.Product, int, error) {
	// 生成缓存键
	cacheKey := r.generateCacheKey(userID, req)
	
	// 尝试从缓存获取数据
	if products, total, found := r.getFromCache(cacheKey); found {
		return products, total, nil
	}
	
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

	// 将结果存入缓存
	r.setToCache(cacheKey, products, total)

	return products, total, nil
}
