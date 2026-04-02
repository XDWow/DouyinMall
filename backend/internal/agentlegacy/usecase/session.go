//go:build legacy_agent

package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
)

// 浼氳瘽绠＄悊
type SessionUseCase struct {
	sessionRepo domain.SessionRepo
}

func NewSessionUseCase(sessionRepo domain.SessionRepo) *SessionUseCase {
	return &SessionUseCase{sessionRepo: sessionRepo}
}

// Create 鍒涘缓鏂颁細璇?
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

// 鑾峰彇瀵硅瘽鍘嗗彶
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

// 鑾峰彇鐢ㄦ埛浼氳瘽鍒楄〃
func (uc *SessionUseCase) ListSessions(ctx context.Context, userID int64, limit, offset int) ([]domain.Session, int, error) {
	return uc.sessionRepo.ListByUser(ctx, userID, limit, offset)
}

// 娓呯┖浼氳瘽
func (uc *SessionUseCase) Clear(ctx context.Context, sessionID string) error {
	return uc.sessionRepo.Clear(ctx, sessionID)
}

// ==================== 浜哄伐瀹㈡湇闃舵鎺ュ彛锛圥hase 2锛?====================
// 浠ヤ笅鎺ュ彛渚涗汉宸ュ鏈嶅伐浣滃彴璋冪敤锛屽綋鍓嶄粎棰勭暀妗?
//
// 鏁翠綋鏋舵瀯锛?
//   鐢ㄦ埛濮嬬粓閫氳繃 Agent 鏈嶅姟鍙戦€佹秷鎭紙鍚屼竴涓叆鍙ｏ紝鍚屼竴涓?session锛?
//   session.Status == Human 鏃讹紝ChatUseCase 鎷︽埅 AI Pipeline锛屼粎瀛樺偍鐢ㄦ埛娑堟伅
//   浜哄伐鍧愬腑閫氳繃宸ヤ綔鍙板墠绔煡鐪嬫秷鎭祦锛岃皟鐢ㄤ互涓嬫帴鍙ｅ啓鍏ュ洖澶嶅拰缁撴潫浼氳瘽
//   娑堟伅璇诲啓鍏ㄩ儴缁忚繃 Agent 鏈嶅姟锛屼繚璇佷竴浠芥暟鎹簮
//
// 娑堟伅娴侊細
//   鐢ㄦ埛 鈫?Agent.SendMessage 鈫?persistUserMessage 鈫?鎺ㄩ€佺粰鍧愬腑宸ヤ綔鍙?
//   鍧愬腑 鈫?Agent.SendHumanReply 鈫?瀛?assistant 娑堟伅 鈫?鎺ㄩ€佺粰鐢ㄦ埛鍓嶇
//   鍧愬腑 鈫?Agent.CloseSession 鈫?Status=Closed 鈫?FlushSession

// SendHumanReply 浜哄伐鍧愬腑鍥炲鐢ㄦ埛
// 鍧愬腑宸ヤ綔鍙拌皟鐢紝鍐欏叆涓€鏉?Role=assistant 鐨勬秷鎭苟鎺ㄩ€佺粰鐢ㄦ埛
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
	// TODO Phase 2: 閫氳繃 WebSocket/SSE 鎺ㄩ€佺粰鐢ㄦ埛鍓嶇
}

// 缁撴潫浜哄伐浼氳瘽
// 鍧愬腑鐐瑰嚮"缁撴潫浼氳瘽"鏃惰皟鐢紝鍒峰啓缁堟€佸埌 MySQL
func (uc *SessionUseCase) CloseSession(ctx context.Context, sessionID string) error {
	session, err := uc.sessionRepo.LoadSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	session.Status = domain.SessionClosed
	session.UpdatedAt = time.Now()
	return uc.sessionRepo.FlushSession(ctx, session)
}
