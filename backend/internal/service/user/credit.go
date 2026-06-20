package user

import (
	"errors"
	"fmt"
	"time"

	"github.com/aikuaixia/aikuaixia/internal/model"
	"github.com/aikuaixia/aikuaixia/internal/pkg/idgen"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CreditService struct {
	db *gorm.DB
}

func NewCreditService(db *gorm.DB) *CreditService {
	return &CreditService{db: db}
}

// AddCredits 增加积分
func (s *CreditService) AddCredits(userID uuid.UUID, amount int, source string, refID *uuid.UUID, description string) error {
	if amount <= 0 {
		return errors.New("积分数量必须大于0")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 查询积分账户
		var credits model.UserCredits
		if err := tx.Where("user_id = ?", userID).First(&credits).Error; err != nil {
			return err
		}

		// 更新积分
		credits.Balance += amount
		credits.TotalRecharged += amount

		if err := tx.Save(&credits).Error; err != nil {
			return err
		}

		// 写入流水
		tx.Create(&model.CreditTransaction{
			UserID:      userID,
			Type:        "recharge",
			Amount:      amount,
			Balance:     credits.Balance,
			Source:      source,
			RefID:       refID,
			Description: description,
		})

		return nil
	})
}

// DeductCredits 扣除积分
func (s *CreditService) DeductCredits(userID uuid.UUID, amount int, source string, refID *uuid.UUID) (int, error) {
	if amount <= 0 {
		return 0, errors.New("积分数量必须大于0")
	}

	var newBalance int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 查询积分账户（加锁）
		var credits model.UserCredits
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ?", userID).First(&credits).Error; err != nil {
			return err
		}

		if credits.Balance < amount {
			return fmt.Errorf("积分不足，当前积分: %d，需要: %d", credits.Balance, amount)
		}

		// 扣除积分
		credits.Balance -= amount
		credits.TotalConsumed += amount
		newBalance = credits.Balance

		if err := tx.Save(&credits).Error; err != nil {
			return err
		}

		// 写入流水
		tx.Create(&model.CreditTransaction{
			UserID:      userID,
			Type:        "consume",
			Amount:      -amount,
			Balance:     credits.Balance,
			Source:      source,
			RefID:       refID,
			Description: fmt.Sprintf("%s 扣除 %d 积分", source, amount),
		})

		return nil
	})

	return newBalance, err
}

// DB 暴露 gorm.DB（仅用于内部 Handler 查询）
func (s *CreditService) DB() *gorm.DB {
	return s.db
}

// GetBalance 获取积分余额
func (s *CreditService) GetBalance(userID uuid.UUID) (int, error) {
	var credits model.UserCredits
	if err := s.db.Where("user_id = ?", userID).First(&credits).Error; err != nil {
		return 0, err
	}
	return credits.Balance, nil
}

// GetTransactions 获取积分流水
func (s *CreditService) GetTransactions(userID uuid.UUID, page, pageSize int) ([]model.CreditTransaction, int64, error) {
	var transactions []model.CreditTransaction
	var total int64

	query := s.db.Where("user_id = ?", userID)
	query.Model(&model.CreditTransaction{}).Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&transactions).Error; err != nil {
		return nil, 0, err
	}

	return transactions, total, nil
}

// CreateRechargeOrder 创建充值订单
func (s *CreditService) CreateRechargeOrder(userID uuid.UUID, packageID, paymentMethod string) (*model.RechargeOrder, error) {
	// 查询套餐
	var pkg model.CreditPackage
	if err := s.db.Where("id = ? AND is_active = true", packageID).First(&pkg).Error; err != nil {
		return nil, errors.New("套餐不存在")
	}

	order := &model.RechargeOrder{
		UserID:        userID,
		OrderNo:       idgen.GenerateRechargeOrderNo(),
		PackageID:     packageID,
		Amount:        pkg.Price,
		Credits:       pkg.Credits + pkg.Bonus,
		PaymentMethod: paymentMethod,
		Status:        "pending",
	}

	if err := s.db.Create(order).Error; err != nil {
		return nil, err
	}

	return order, nil
}

// CompleteRechargeOrder 完成充值订单（支付回调）
func (s *CreditService) CompleteRechargeOrder(orderNo, paymentNo string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var order model.RechargeOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return errors.New("订单不存在")
		}

		if order.Status != "pending" {
			return errors.New("订单状态异常")
		}

		// 更新订单状态
		now := time.Now()
		order.Status = "paid"
		order.PaymentNo = paymentNo
		order.PaidAt = &now

		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		// 增加积分
		var credits model.UserCredits
		if err := tx.Where("user_id = ?", order.UserID).First(&credits).Error; err != nil {
			return err
		}

		credits.Balance += order.Credits
		credits.TotalRecharged += order.Credits

		if err := tx.Save(&credits).Error; err != nil {
			return err
		}

		// 写入积分流水
		tx.Create(&model.CreditTransaction{
			UserID:      order.UserID,
			Type:        "recharge",
			Amount:      order.Credits,
			Balance:     credits.Balance,
			Source:      "recharge",
			RefID:       &order.ID,
			Description: fmt.Sprintf("充值 %v 元，获得 %d 积分", order.Amount, order.Credits),
		})

		return nil
	})
}

// GetRechargeOrders 获取充值订单列表
func (s *CreditService) GetRechargeOrders(userID uuid.UUID, page, pageSize int) ([]model.RechargeOrder, int64, error) {
	var orders []model.RechargeOrder
	var total int64

	query := s.db.Where("user_id = ?", userID)
	query.Model(&model.RechargeOrder{}).Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}
