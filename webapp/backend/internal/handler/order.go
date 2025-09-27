package handler

import (
    "backend/internal/middleware"
    "backend/internal/model"
    "backend/internal/service"
    "encoding/json"
    "log"
    "net/http"
)

type OrderHandler struct {
	OrderSvc *service.OrderService
}

func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{OrderSvc: svc}
}

// 注文履歴一覧を取得
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	var req model.ListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

    req.Normalize(model.ListRequestConfig{
        DefaultPage:       1,
        DefaultPageSize:   20,
        MaxPageSize:       100,
        DefaultSortField:  "order_id",
        DefaultSortOrder:  "DESC",
        AllowedSortFields: []string{"order_id", "product_id", "created_at", "shipped_status", "arrived_at"},
        AllowedSortOrders: []string{"ASC", "DESC"},
        DefaultType:       "partial",
        AllowedTypes:      []string{"partial", "prefix"},
    })

	orders, total, err := h.OrderSvc.FetchOrders(r.Context(), userID, req)
	if err != nil {
		log.Printf("Failed to fetch orders for user %d: %v", userID, err)
		http.Error(w, "Failed to fetch orders", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Data  []model.Order `json:"data"`
		Total int           `json:"total"`
	}{
		Data:  orders,
		Total: total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
