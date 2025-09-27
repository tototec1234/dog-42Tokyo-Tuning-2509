package service

import (
	"context"
	"log"
    "fmt"

	"backend/internal/model"
	"backend/internal/repository"

	"go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

type ProductService struct {
	store *repository.Store
}

func NewProductService(store *repository.Store) *ProductService {
	return &ProductService{store: store}
}

func (s *ProductService) CreateOrders(ctx context.Context, userID int, items []model.RequestItem) ([]string, error) {

	tracer := otel.Tracer("app/custom")
   ctx, span := tracer.Start(ctx, "CreateOrders")
   defer span.End()
   span.SetAttributes(attribute.Int("user.id", userID), attribute.Int("items.count", len(items)))

    var insertedOrderIDs []string

    err := s.store.ExecTx(ctx, func(txStore *repository.Store) error {
        // 聚合相同 product_id 的數量
        itemsToProcess := make(map[int]int)
        for _, item := range items {
            if item.Quantity > 0 {
                itemsToProcess[item.ProductID] += item.Quantity
            }
        }
        if len(itemsToProcess) == 0 {
            return nil
        }

        // 對每個 product 批量插入，避免逐筆 INSERT
        for productID, quantity := range itemsToProcess {
            ids, err := txStore.OrderRepo.CreateBulk(ctx, userID, productID, quantity)
            if err != nil {
                return err
            }
            for _, id := range ids {
                insertedOrderIDs = append(insertedOrderIDs, fmt.Sprintf("%d", id))
            }
        }
        return nil
    })

    if err != nil {
        return nil, err
    }
    log.Printf("Created %d orders for user %d", len(insertedOrderIDs), userID)
    return insertedOrderIDs, nil
}

func (s *ProductService) FetchProducts(ctx context.Context, userID int, req model.ListRequest) ([]model.Product, int, error) {
	products, total, err := s.store.ProductRepo.ListProducts(ctx, userID, req)
	return products, total, err
}
