package service

import (
    "context"
    "fmt"
    "log"

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

    aggregated := aggregateItems(items)
    if len(aggregated) == 0 {
        return nil, nil
    }

    insertedOrderIDs := make([]string, 0, len(items))

    err := s.store.ExecTx(ctx, func(txStore *repository.Store) error {
        bulkInsertFn := func(productID int, quantity int) error {
            ids, err := txStore.OrderRepo.CreateBulk(ctx, userID, productID, quantity)
            if err != nil {
                return err
            }
            for _, id := range ids {
                insertedOrderIDs = append(insertedOrderIDs, fmt.Sprintf("%d", id))
            }
            return nil
        }

        return processInBatches(ctx, aggregated, bulkInsertFn)
    })

    if err != nil {
        return nil, err
    }
    log.Printf("Created %d orders for user %d", len(insertedOrderIDs), userID)
    return insertedOrderIDs, nil
}

func aggregateItems(items []model.RequestItem) map[int]int {
    aggregated := make(map[int]int)
    for _, item := range items {
        if item.Quantity > 0 {
            aggregated[item.ProductID] += item.Quantity
        }
    }
    return aggregated
}

func processInBatches(ctx context.Context, items map[int]int, fn func(productID int, quantity int) error) error {
    const batchSize = 500
    type entry struct {
        productID int
        quantity  int
    }

    entries := make([]entry, 0, len(items))
    for productID, quantity := range items {
        entries = append(entries, entry{productID: productID, quantity: quantity})
    }

    for i := 0; i < len(entries); i += batchSize {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        end := i + batchSize
        if end > len(entries) {
            end = len(entries)
        }

        batch := entries[i:end]
        for _, item := range batch {
            if err := fn(item.productID, item.quantity); err != nil {
                return err
            }
        }
    }

    return nil
}

func (s *ProductService) FetchProducts(ctx context.Context, userID int, req model.ListRequest) ([]model.Product, int, error) {
	products, total, err := s.store.ProductRepo.ListProducts(ctx, userID, req)
	return products, total, err
}
