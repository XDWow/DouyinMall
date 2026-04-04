//go:build legacy_agent

package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
)

// 娴兼俺鐦界粻锛勬倞
type SessionUseCase struct {
	sessionRepo domain.SessionRepo
}

func NewSessionUseCase(sessionRepo domain.SessionRepo) *SessionUseCase {
	return &SessionUseCase{sessionRepo: sessionRepo}
}

// Create 閸掓稑缂撻弬棰佺窗鐠?
func (uc *SessionUseCase) Create(ctx context.Context, userID int64, channel string) (*domain.Session, error) {
	session := &domain.Session{
		ID:        fmt.Sprintf("sess_%d_%d", userID, time.Now().UnixMilli()),
		UserID:    userID,
		Channel:   channel,
		Status:    domain.SessionActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := uc.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

// 閼惧嘲褰囩€电鐦介崢鍡楀蕉
func (uc *SessionUseCase) GetHistory(ctx context.Context, sessionID string, limit, offset int) ([]domain.Message, int, error) {
	msgs, err := uc.sessionRepo.LoadMessages(ctx, sessionID)
	if err != nil {
		return nil, 0, fmt.Errorf("load messages: %w", err)
	}
	total := len(msgs)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return msgs[offset:end], total, nil
}

// 閼惧嘲褰囬悽銊﹀煕娴兼俺鐦介崚妤勩€?func (uc *SessionUseCase) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	return uc.sessionRepo.ListByUser(ctx, userID, limit, offset)
}

// 濞撳懐鈹栨导姘崇樈
func (uc *SessionUseCase) Clear(ctx context.Context, sessionID string) error {
	return uc.sessionRepo.Clear(ctx, sessionID)
}

// ==================== 娴滃搫浼愮€广垺婀囬梼鑸殿唽閹恒儱褰涢敍鍦ase 2閿?====================
// 娴犮儰绗呴幒銉ュ經娓氭稐姹夊銉ヮ吂閺堝秴浼愭担婊冨酱鐠嬪啰鏁ら敍灞界秼閸撳秳绮庢０鍕殌濡?
//
// 閺佺繝缍嬮弸鑸电€敍?
//   閻劍鍩涙慨瀣矒闁俺绻?Agent 閺堝秴濮熼崣鎴︹偓浣圭Х閹垽绱欓崥灞肩娑擃亜鍙嗛崣锝忕礉閸氬奔绔存稉?session閿?
//   session.Status == Human 閺冭绱滳hatUseCase 閹凤附鍩?AI Pipeline閿涘奔绮庣€涙ê鍋嶉悽銊﹀煕濞戝牊浼?//   娴滃搫浼愰崸鎰厬闁俺绻冨銉ょ稊閸欐澘澧犵粩顖涚叀閻绉烽幁顖涚ウ閿涘矁鐨熼悽銊や簰娑撳甯撮崣锝呭晸閸忋儱娲栨径宥呮嫲缂佹挻娼导姘崇樈
//   濞戝牊浼呯拠璇插晸閸忋劑鍎寸紒蹇氱箖 Agent 閺堝秴濮熼敍灞肩箽鐠囦椒绔存禒鑺ユ殶閹诡喗绨?//
// 濞戝牊浼呭ù渚婄窗
//   閻劍鍩?閳?Agent.SendMessage 閳?persistUserMessage 閳?閹恒劑鈧胶绮伴崸鎰厬瀹搞儰缍旈崣?
//   閸ф劕鑵?閳?Agent.SendHumanReply 閳?鐎?assistant 濞戝牊浼?閳?閹恒劑鈧胶绮伴悽銊﹀煕閸撳秶顏?//   閸ф劕鑵?閳?Agent.CloseSession 閳?Status=Closed 閳?FlushSession

// SendHumanReply 娴滃搫浼愰崸鎰厬閸ョ偛顦查悽銊﹀煕
// 閸ф劕鑵戝銉ょ稊閸欐媽鐨熼悽顭掔礉閸愭瑥鍙嗘稉鈧弶?Role=assistant 閻ㄥ嫭绉烽幁顖氳嫙閹恒劑鈧胶绮伴悽銊﹀煕
func (uc *SessionUseCase) SendHumanReply(ctx context.Context, sessionID string, content string) error {
	session, err := uc.sessionRepo.LoadSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if session.Status != domain.SessionHuman {
		return fmt.Errorf("session %s is not in human mode", sessionID)
	}
	msg := domain.Message{
		SessionID: sessionID,
		Role:      domain.RoleAssistant,
		Content:   content,
		CreatedAt: time.Now(),
	}
	return uc.sessionRepo.AppendMessages(ctx, session, []domain.Message{msg})
	// TODO Phase 2: 闁俺绻?WebSocket/SSE 閹恒劑鈧胶绮伴悽銊﹀煕閸撳秶顏?}

// 缂佹挻娼禍鍝勪紣娴兼俺鐦?// 閸ф劕鑵戦悙鐟板毊"缂佹挻娼导姘崇樈"閺冩儼鐨熼悽顭掔礉閸掑嘲鍟撶紒鍫熲偓浣稿煂 MySQL
func (uc *SessionUseCase) CloseSession(ctx context.Context, sessionID string) error {
	session, err := uc.sessionRepo.LoadSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	session.Status = domain.SessionClosed
	session.UpdatedAt = time.Now()
	return uc.sessionRepo.FlushSession(ctx, session)
}


