package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/coupon/domain"
)

// ==================== 棰嗗埜鐢ㄤ緥 ====================

type IssueCouponUseCase struct {
	templateRepo  domain.CouponTemplateRepository
	couponRepo    domain.CouponRepository
	operationRepo domain.CouponOperationRepository
}

func NewIssueCouponUseCase(
	templateRepo domain.CouponTemplateRepository,
	couponRepo domain.CouponRepository,
	operationRepo domain.CouponOperationRepository,
) *IssueCouponUseCase {
	return &IssueCouponUseCase{
		templateRepo:  templateRepo,
		couponRepo:    couponRepo,
		operationRepo: operationRepo,
	}
}

type IssueCouponInput struct {
	UserID      int64
	TemplateID  int64
	OperationID string // 骞傜瓑閿紙蹇呴』甯?coupon: 鍓嶇紑锛?}

type IssueCouponOutput struct {
	CouponID int64 // 涓€娆″彧鍙戜竴寮犲埜
}

func (uc *IssueCouponUseCase) Execute(ctx context.Context, input IssueCouponInput) (*IssueCouponOutput, error) {
	// 鏍￠獙鍙傛暟
	if input.UserID <= 0 {
		return nil, errors.New("invalid user_id")
	}
	if input.TemplateID <= 0 {
		return nil, errors.New("invalid template_id")
	}
	if input.OperationID == "" {
		return nil, errors.New("operation_id is required")
	}

	// 1. 骞傜瓑妫€鏌ワ細鏌ヨ鏄惁宸插彂鏀捐繃
	operation, err := uc.operationRepo.GetByOperationID(ctx, input.OperationID)
	if err != nil && err != domain.ErrOperationNotFound {
		return nil, err
	}
	// 濡傛灉宸插彂鏀撅紝鐩存帴杩斿洖涔嬪墠鐨勫埜ID锛堝箓绛夛級
	if operation != nil {
		return &IssueCouponOutput{CouponID: operation.UserCouponID}, nil
	}

	// 2. 妫€鏌ユā鏉挎槸鍚﹀彲鍙戞斁
	template, err := uc.templateRepo.GetByID(ctx, input.TemplateID)
	if err != nil {
		return nil, err
	}
	if !template.CanIssue() {
		return nil, domain.ErrCouponCannotIssue
	}

	// 3. 妫€鏌ョ敤鎴烽檺棰嗘暟閲?	count, err := uc.couponRepo.CountByUserAndTemplate(ctx, input.UserID, input.TemplateID)
	if err != nil {
		return nil, err
	}
	if count >= template.PerUserLimit {
		return nil, domain.ErrCouponLimitExceeded
	}

	// 4. 鍙戞斁鐢ㄦ埛浼樻儬鍒革紙涓€娆″彧鍙戜竴寮狅級
	validFrom, validTo := template.CalculateValidTime()
	coupon := &domain.Coupon{
		UserID:         input.UserID,
		TemplateID:     input.TemplateID,
		Status:         domain.UserCouponStatusUnused,
		ValidStartTime: time.Unix(validFrom, 0),
		ValidEndTime:   time.Unix(validTo, 0),
		Template:       &template,
	}
	couponID, err := uc.couponRepo.Issue(ctx, coupon)
	if err != nil {
		return nil, err
	}

	// 5. 璁板綍骞傜瓑鎿嶄綔锛堝繀椤绘垚鍔燂紝鍚﹀垯褰卞搷骞傜瓑鎬э級
	if err := uc.operationRepo.Create(ctx, &domain.CouponOperation{
		OperationID:  input.OperationID,
		UserCouponID: couponID,
		Type:         "ISSUE",
	}); err != nil {
		// TODO: 杩欓噷澶辫触搴旇鍥炴粴鍒哥殑鍙戞斁锛屽疄闄呭簲璇ュ湪浜嬪姟涓畬鎴?		// 褰撳墠瀹炵幇锛氬鏋滃け璐ワ紝涓嬫閲嶈瘯鏃朵細鍥犱负鍒稿凡瀛樺湪瀵艰嚧閲嶅鍙戝埜
		return nil, err
	}

	// 6. 澧炲姞妯℃澘宸插彂鏀炬暟閲忥紙闈炲己涓€鑷达紝鍏佽灏戦噺瓒呭彂锛?	if err := uc.templateRepo.IncrIssuedCount(ctx, input.TemplateID); err != nil {
		// 杩欓噷澶辫触涓嶅洖婊氬埜鐨勫彂鏀撅紝鎺ュ彈灏戦噺瓒呭彂
		// 鍥犱负璁℃暟涓嶅噯纭殑褰卞搷 < 鍙戝埜澶辫触鐨勫奖鍝?	}

	return &IssueCouponOutput{CouponID: couponID}, nil
}


