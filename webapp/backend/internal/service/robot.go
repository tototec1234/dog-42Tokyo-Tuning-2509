package service

import (
	"backend/internal/model"
	"backend/internal/repository"
	"backend/internal/service/utils"
	"context"
	"log"
)

type RobotService struct {
	store *repository.Store
}

func NewRobotService(store *repository.Store) *RobotService {
	return &RobotService{store: store}
}

func (s *RobotService) GenerateDeliveryPlan(ctx context.Context, robotID string, capacity int) (*model.DeliveryPlan, error) {
	var plan model.DeliveryPlan

	err := utils.WithTimeout(ctx, func(ctx context.Context) error {
		return s.store.ExecTx(ctx, func(txStore *repository.Store) error {
			orders, err := txStore.OrderRepo.GetShippingOrders(ctx)
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

func selectOrdersForDelivery(ctx context.Context, orders []model.Order, robotID string, robotCapacity int) (model.DeliveryPlan, error) {
	// 動的計画法を使用して0-1ナップサック問題を効率的に解決
	n := len(orders)
	if n == 0 {
		return model.DeliveryPlan{RobotID: robotID, TotalWeight: 0, TotalValue: 0, Orders: []model.Order{}}, nil
	}

	// DPテーブル: dp[i][w] = 最初のi個のアイテムから重さw以内で得られる最大価値
	// メモリ効率のために1次元配列を使用
	dp := make([]int, robotCapacity+1)
	selected := make([][]bool, n)
	for i := range selected {
		selected[i] = make([]bool, robotCapacity+1)
	}

	// DP計算
	for i := 0; i < n; i++ {
		order := orders[i]
		// 逆順に更新して、同じアイテムを複数回使わないようにする
		for w := robotCapacity; w >= order.Weight; w-- {
			if dp[w-order.Weight] + order.Value > dp[w] {
				dp[w] = dp[w-order.Weight] + order.Value
				selected[i][w] = true
			}
		}

		// 定期的にコンテキストのキャンセルを確認
		if i%100 == 0 {
			select {
			case <-ctx.Done():
				return model.DeliveryPlan{}, ctx.Err()
			default:
			}
		}
	}

	// 最適解を復元
	var bestSet []model.Order
	w := robotCapacity
	totalWeight := 0
	totalValue := dp[robotCapacity]

	for i := n - 1; i >= 0; i-- {
		if selected[i][w] {
			order := orders[i]
			bestSet = append(bestSet, order)
			totalWeight += order.Weight
			w -= order.Weight
		}
	}

	return model.DeliveryPlan{
		RobotID:     robotID,
		TotalWeight: totalWeight,
		TotalValue:  totalValue,
		Orders:      bestSet,
	}, nil
}
