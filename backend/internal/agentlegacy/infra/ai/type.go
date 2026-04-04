//go:build legacy_agent

package ai

import (
	"context"

	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
)

// 鏉╂瑩鍣烽弰顖欒礋娑撴艾濮熺€圭偟骞?ai chat閿涘矁鐨熼悽銊ョ俺鐏?pkg/ai閿涘本澹欐稉濠傛儙娑?
// 閸欘垯浜掗崑姘辨畱娴滃鍎忛弰顖ょ窗闂堛垹鎮滄稉姘閿涘瞼绮版稉姘閺嗘挳婀剁粻鈧崡鏇犳畱閹恒儱褰涢敍宀€鍔ч崥搴ゅ殰瀹歌精藟閸忓懍绔存禍娑樻祼鐎规矮淇婇幁顖ょ礉濮ｆ柨顩уΟ鈥崇€烽柅澶嬪閿涘矂鍘ょ純顕嗙礉娑撴艾濮熼弬閫涚瑝閻劌鍙ц箛鍐箹娑?
// 鏉╂ɑ婀侀敍灞剧ウ瀵繒娈戦弮璺衡偓娆戞畱tool calling 閻ㄥ嫬寮弫鐗堟暪闂嗗棴绱濋弨鑸靛姬娴滃棗鍟€娴肩姷绮版稉姘閺傜鐨熼悽?mcp server
// 娴犮儱寮烽張宥呭濞岃崵鎮婇敍宀€鍟嶉弬顓ㄧ礉闂勬劖绁﹂敍宀勬缁?
type CSLLMClient interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error)
}

// 娑撴艾濮熼弬纭咁洣娴肩姳绮堟稊鍫吹缁犫偓閸栨牗甯撮崣锝忕礉闂呮劘妫屾惔鏇炵湴缂佸棜濡?// Model閵嗕箑emperature閵嗕府axTokens 缁涘寮弫鎵暠 infra 鐏炲倻绮烘稉鈧柊宥囩枂
type ChatRequest struct {
	Messages []pkgai.Message `json:"messages"`
	Tools    []pkgai.ToolDef `json:"tools,omitempty"`
}

// 娑撴艾濮熼弬褰掓付鐟曚焦瀣侀崚棰佺矆娑斿牞绱?type ChatResponse struct {
	ID         string         `json:"id"`
	Created    int64          `json:"created"`
	Choices    []pkgai.Choice `json:"choices"`
	TokensUsed int            `json:"tokens_used,omitempty"` // 閻劋绨拋鏉跨秿閸滃瞼娲冮幒?
}


