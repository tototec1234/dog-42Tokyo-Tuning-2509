package service

import (
	"backend/internal/model"
	"backend/internal/repository"
	"backend/internal/service/utils"
	"context"
	"log"
	"math"
	"sort"
)

type RobotService struct {
	store *repository.Store
}

func NewRobotService(store *repository.Store) *RobotService {
	return &RobotService{store: store}
}

func (s *RobotService) GenerateDeliveryPlan(ctx context.Context, robotID string, capacity int) (*model.DeliveryPlan, error) {
	if capacity <= 0 {
		return &model.DeliveryPlan{RobotID: robotID, Orders: []model.Order{}}, nil
	}

	var plan model.DeliveryPlan

	const fetchLimit = 2048

	err := utils.WithTimeout(ctx, func(ctx context.Context) error {
		return s.store.ExecTx(ctx, func(txStore *repository.Store) error {
            orders, err := txStore.OrderRepo.GetShippingOrders(ctx, fetchLimit, capacity)
			if err != nil {
				return err
			}

			plan, err = selectOrdersForDelivery(ctx, orders, robotID, capacity)
			if err != nil {
				return err
			}

			if len(plan.Orders) > 0 {
				orderIDs := make([]int64, len(plan.Orders))
				for i, order := range plan.Orders {
					orderIDs[i] = order.OrderID
				}

				if err := txStore.OrderRepo.UpdateStatuses(ctx, orderIDs, "delivering"); err != nil {
					return err
				}
				log.Printf("Updated status to 'delivering' for %d orders", len(orderIDs))
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (s *RobotService) UpdateOrderStatus(ctx context.Context, orderID int64, newStatus string) error {
	return utils.WithTimeout(ctx, func(ctx context.Context) error {
		return s.store.OrderRepo.UpdateStatuses(ctx, []int64{orderID}, newStatus)
	})
}

const (
	contextCheckInterval = 96
	maxOrdersForDP       = 1600
)

func selectOrdersForDelivery(ctx context.Context, orders []model.Order, robotID string, robotCapacity int) (model.DeliveryPlan, error) {
	if robotCapacity <= 0 {
		return model.DeliveryPlan{RobotID: robotID, Orders: []model.Order{}}, nil
	}

	filtered := make([]model.Order, 0, len(orders))
	for _, order := range orders {
		if order.Weight <= 0 || order.Weight > robotCapacity {
			continue
		}
		filtered = append(filtered, order)
	}

	if len(filtered) == 0 {
		return model.DeliveryPlan{RobotID: robotID, Orders: []model.Order{}}, nil
	}

	sort.Slice(filtered, func(i, j int) bool {
		leftWeight := math.Max(1, float64(filtered[i].Weight))
		rightWeight := math.Max(1, float64(filtered[j].Weight))
		leftDensity := float64(filtered[i].Value) / leftWeight
		rightDensity := float64(filtered[j].Value) / rightWeight
		if leftDensity == rightDensity {
			if filtered[i].Value == filtered[j].Value {
				return filtered[i].OrderID < filtered[j].OrderID
			}
			return filtered[i].Value > filtered[j].Value
		}
		return leftDensity > rightDensity
	})

	if len(filtered) > maxOrdersForDP {
		filtered = filtered[:maxOrdersForDP]
	}

	capacity := robotCapacity
	dp := make([]int, capacity+1)
	parent := make([]int, capacity+1)
	choice := make([]int, capacity+1)
	for i := 0; i <= capacity; i++ {
		parent[i] = -1
		choice[i] = -1
	}

	for idx, order := range filtered {
		weight := order.Weight
		value := order.Value
		if weight <= 0 {
			continue
		}
		for w := capacity; w >= weight; w-- {
			candidate := dp[w-weight] + value
			if candidate > dp[w] {
				dp[w] = candidate
				parent[w] = w - weight
				choice[w] = idx
			}
		}

		if idx%contextCheckInterval == 0 {
			select {
			case <-ctx.Done():
				return model.DeliveryPlan{}, ctx.Err()
			default:
			}
		}
	}

	bestValue := 0
	bestWeight := 0
	for w := 0; w <= capacity; w++ {
		if dp[w] > bestValue {
			bestValue = dp[w]
			bestWeight = w
		}
	}

	if bestValue <= 0 {
		return model.DeliveryPlan{RobotID: robotID, Orders: []model.Order{}}, nil
	}

	bestSet := make([]model.Order, 0)
	weightCursor := bestWeight
	for weightCursor > 0 && choice[weightCursor] != -1 {
		idx := choice[weightCursor]
		bestSet = append(bestSet, filtered[idx])
		weightCursor = parent[weightCursor]
	}

	for i, j := 0, len(bestSet)-1; i < j; i, j = i+1, j-1 {
		bestSet[i], bestSet[j] = bestSet[j], bestSet[i]
	}

	totalWeight := 0
	for _, order := range bestSet {
		totalWeight += order.Weight
	}

	sanitizedOrders := make([]model.Order, len(bestSet))
	for i, order := range bestSet {
		sanitizedOrders[i] = model.Order{
			OrderID: order.OrderID,
			Weight:  order.Weight,
			Value:   order.Value,
		}
	}

	return model.DeliveryPlan{
		RobotID:     robotID,
		TotalWeight: totalWeight,
		TotalValue:  bestValue,
		Orders:      sanitizedOrders,
	}, nil
}
