//go:build legacy_agent

package ai

import (
	"context"

	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
)

// 杩欓噷鏄负涓氬姟瀹炵幇 ai chat锛岃皟鐢ㄥ簳灞?pkg/ai锛屾壙涓婂惎涓?
// 鍙互鍋氱殑浜嬫儏鏄細闈㈠悜涓氬姟锛岀粰涓氬姟鏆撮湶绠€鍗曠殑鎺ュ彛锛岀劧鍚庤嚜宸辫ˉ鍏呬竴浜涘浐瀹氫俊鎭紝姣斿妯″瀷閫夋嫨锛岄厤缃紝涓氬姟鏂逛笉鐢ㄥ叧蹇冭繖涓?
// 杩樻湁锛屾祦寮忕殑鏃跺€欑殑tool calling 鐨勫弬鏁版敹闆嗭紝鏀舵弧浜嗗啀浼犵粰涓氬姟鏂硅皟鐢?mcp server
// 浠ュ強鏈嶅姟娌荤悊锛岀啍鏂紝闄愭祦锛岄檷绾?
type CSLLMClient interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error)
}

// 涓氬姟鏂硅浼犱粈涔堬紵绠€鍖栨帴鍙ｏ紝闅愯棌搴曞眰缁嗚妭
// Model銆乀emperature銆丮axTokens 绛夊弬鏁扮敱 infra 灞傜粺涓€閰嶇疆
type ChatRequest struct {
	Messages []pkgai.Message `json:"messages"`
	Tools    []pkgai.ToolDef `json:"tools,omitempty"`
}

// 涓氬姟鏂归渶瑕佹嬁鍒颁粈涔堬紵
type ChatResponse struct {
	ID         string         `json:"id"`
	Created    int64          `json:"created"`
	Choices    []pkgai.Choice `json:"choices"`
	TokensUsed int            `json:"tokens_used,omitempty"` // 鐢ㄤ簬璁板綍鍜岀洃鎺?
}
