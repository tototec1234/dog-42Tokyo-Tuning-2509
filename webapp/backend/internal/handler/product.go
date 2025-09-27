package handler

import (
	"backend/internal/middleware"
	"backend/internal/model"
	"backend/internal/service"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type ProductHandler struct {
	ProductSvc *service.ProductService
}

func NewProductHandler(svc *service.ProductService) *ProductHandler {
	return &ProductHandler{ProductSvc: svc}
}

// 商品一覧を取得
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	var req model.ListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
    if req.PageSize <= 0 {
        req.PageSize = 20
    }
    if req.PageSize > 100 {
        req.PageSize = 100
    }
    if req.SortField == "" {
        req.SortField = "product_id"
    }
    // 限制排序欄位白名單
    switch req.SortField {
    case "product_id", "name", "value", "weight":
        // ok
    default:
        req.SortField = "product_id"
    }
    if req.SortOrder == "" {
        req.SortOrder = "asc"
    }
    if req.SortOrder != "asc" && req.SortOrder != "desc" {
        req.SortOrder = "asc"
    }
	req.Offset = (req.Page - 1) * req.PageSize

	products, total, err := h.ProductSvc.FetchProducts(r.Context(), userID, req)
	if err != nil {
		log.Printf("Failed to fetch products for user %d: %v", userID, err)
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Data  []model.Product `json:"data"`
		Total int             `json:"total"`
	}{
		Data:  products,
		Total: total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// 注文を作成
func (h *ProductHandler) CreateOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusInternalServerError)
		return
	}

	var req model.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	insertedOrderIDs, err := h.ProductSvc.CreateOrders(r.Context(), userID, req.Items)
	if err != nil {
		log.Printf("Failed to create orders: %v", err)
		http.Error(w, "Failed to process order request", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message":   "Orders created successfully",
		"order_ids": insertedOrderIDs,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *ProductHandler) GetImage(w http.ResponseWriter, r *http.Request) {
    // noisy log を削減（負荷テスト中は大量アクセスされるため）
	imagePath := r.URL.Query().Get("path")
	if imagePath == "" {
		fmt.Println("画像パスが空です")
		http.Error(w, "画像パスが指定されていません", http.StatusBadRequest)
		return
	}

	imagePath = filepath.Clean(imagePath)
	if filepath.IsAbs(imagePath) || strings.Contains(imagePath, "..") {
		fmt.Printf("無効なパス: %s\n", imagePath)
		http.Error(w, "無効なパスです", http.StatusBadRequest)
		return
	}

	baseImageDir := "/app/images"
	fullPath := filepath.Join(baseImageDir, imagePath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		fmt.Printf("画像ファイルが見つかりません: %s\n", fullPath)
		http.Error(w, "画像が見つかりません", http.StatusNotFound)
		return
	}

    // 讓快取生效，Nginx 也會再覆蓋/補強
    w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=604800")
    http.ServeFile(w, r, fullPath)
}
