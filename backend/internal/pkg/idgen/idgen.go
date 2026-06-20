package idgen

import (
	"fmt"
	"time"
	"math/rand"
)

// GenerateOrderNo 生成订单号
func GenerateOrderNo(prefix string) string {
	now := time.Now()
	random := rand.Intn(10000)
	return fmt.Sprintf("%s%s%04d", prefix, now.Format("20060102150405"), random)
}

// GenerateRechargeOrderNo 生成充值订单号
func GenerateRechargeOrderNo() string {
	return GenerateOrderNo("RC")
}

// GenerateConsumeOrderNo 生成消费订单号
func GenerateConsumeOrderNo() string {
	return GenerateOrderNo("CS")
}
