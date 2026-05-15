package alipay

import (
	"fmt"
	"strconv"
	"strings"
)

func centsToYuan(total int64) string {
	sign := ""
	if total < 0 {
		sign = "-"
		total = -total
	}
	return fmt.Sprintf("%s%d.%02d", sign, total/100, total%100)
}

func yuanToCents(amount string) int64 {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return 0
	}
	f, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0
	}
	return int64(f*100 + 0.5)
}
