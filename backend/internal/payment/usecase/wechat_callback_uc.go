package usecase

import (
	"context"
	"errors"
	"github.com/XDWow/DouyinMall/backend/internal/payment/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	orderv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/order/v1/orderservice"
	"strconv"
)

type PayCallbackUC struct {
	repo     domain.PaymentRepository
	orderCli orderservice.Client
	l        logger.LoggerV1

	// 在微信 native 里面，分别是
	// SUCCESS：支付成功
	// REFUND：转入退款
	// NOTPAY：未支付
	// CLOSED：已关闭
	// REVOKED：已撤销（付款码支付）
	// USERPAYING：用户支付中（付款码支付）
	// PAYERROR：支付失败(其他原因，如银行返回失败)
	nativeCBTypeToStatus map[string]domain.PaymentStatus
}

func NewPayCallbackUC(repo domain.PaymentRepository, orderCli orderservice.Client) *PayCallbackUC {
	return &PayCallbackUC{
		repo:     repo,
		orderCli: orderCli,
		nativeCBTypeToStatus: map[string]domain.PaymentStatus{
			// 只有支付成功能推动订单状态为 paid，其他中态不行，支付失败!=订单失败，订单应该继续存在，可以再次尝试支付
			"SUCCESS":  domain.PaymentStatusSuccess,
			"PAYERROR": domain.PaymentStatusFailed,
			// 这个状态，有些人会考虑映射过去 PaymentStatusFailed
			"NOTPAY":     domain.PaymentStatusInit,
			"USERPAYING": domain.PaymentStatusInit,
			"CLOSED":     domain.PaymentStatusFailed,
			"REVOKED":    domain.PaymentStatusFailed,
			"REFUND":     domain.PaymentStatusRefund,
			// 其它状态都可以加
		},
	}
}

// 预扣 + 异步确认
// 防止超卖：
// 预扣：解决并发抢
// 支付后确认：解决最终归属
func (uc *PayCallbackUC) Execute(ctx context.Context, cmd CallbackCmd) error {
	return uc.UpdatePaymentByTxn(ctx, cmd)
}

// UpdatePaymentByTxn 更新支付状态并同步订单状态（可被定时任务等场景复用）
func (uc *PayCallbackUC) UpdatePaymentByTxn(ctx context.Context, cmd CallbackCmd) error {
	status, ok := uc.nativeCBTypeToStatus[cmd.TradeState]
	if !ok {
		return errors.New("未知的微信状态")
	}
	// usecase 决定更新哪些字段，repo 只负责操作
	pmt := domain.Payment{
		BizTradeNo: cmd.OutTradeNo,
		Status:     status,
		TxnID:      cmd.TransactionId,
	}
	err := uc.repo.UpdatePayment(ctx, pmt)
	if err != nil {
		return err
	}

	// 当支付成功，同步调用订单服务更新状态：
	// 其他微服务依赖订单状态来操作，订单状态是后续所有流程的统一判断依据，而不是支付状态
	// 必须确认订单已进入 PAID 状态，才能继续广播支付成功事件，
	// 否则会导致系统间状态不一致。
	if status != domain.PaymentStatusSuccess {
		return nil
	}
	// 通知订单支付成功
	ID, _ := strconv.Atoi(cmd.OutTradeNo)
	orderID := int64(ID)
	_, err = uc.orderCli.ChangeOrderStatus(ctx, &orderv1.ChangeOrderStatusReq{
		OrderId:     orderID,
		OrderStatus: uint32(status.AsUint8()),
	})
	if err != nil {
		uc.l.Error("改变订单状态为已支付失败",
			logger.Error(err),
			logger.Int64("订单号", orderID))
		return err
	}
	return nil
}

type CallbackCmd struct {
	TradeState    string
	TransactionId string
	OutTradeNo    string // 就是业务唯一标识：BizTradeNo，订单中就是 orderID
}
