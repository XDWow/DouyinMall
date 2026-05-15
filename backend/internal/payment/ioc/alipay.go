package ioc

import (
	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	alipayadapter "github.com/XDWow/DouyinMall/backend/internal/payment/infra/alipay"
	"github.com/XDWow/DouyinMall/backend/internal/payment/usecase"
	ali "github.com/smartwalle/alipay/v3"
	"github.com/spf13/viper"
)

func InitPagePayService(client *ali.Client) domain.PagePayService {
	if client == nil {
		return nil
	}
	return alipayadapter.NewPagePayService(client)
}

func InitAlipayWebConfig() usecase.AlipayWebConfig {
	return usecase.AlipayWebConfig{
		AppID:     viper.GetString("alipay.app_id"),
		PID:       viper.GetString("alipay.pid"),
		NotifyURL: viper.GetString("alipay.notify_url"),
		ReturnURL: viper.GetString("alipay.return_url"),
	}.Normalize()
}
