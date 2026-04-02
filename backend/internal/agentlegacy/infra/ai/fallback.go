//go:build legacy_agent

package ai

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// 鏈€灏忓寲鎸囨爣鎺ュ彛锛岀敱 usecase.PipelineMetrics 闅愬紡瀹炵幇
type FallbackMetrics interface {
	IncTemplateFallback()
	IncLLMError()
}

// FallbackLLMClient 闄嶇骇瑁呴グ鍣細鍙礋璐?鑺傜偣A澶辫触鈫掕瘯鑺傜偣B鈫掕瘯妯℃澘"杩欎竴浠朵簨
// 姣忎釜鑺傜偣宸茬敱 ResilientClient 瑁呴グ锛堥檺娴?+ 鐔旀柇 + 瓒呮椂锛夛紝FallbackLLMClient 涓嶆劅鐭ュ叿浣撳師鍥?
type FallbackLLMClient struct {
	nodes    []CSLLMClient // 瑁呴グ ResilientClient
	template *TemplateEngine
	metrics  FallbackMetrics
	logger   logger.LoggerV1
}

func NewFallbackLLMClient(log logger.LoggerV1, metrics FallbackMetrics, nodes ...CSLLMClient) *FallbackLLMClient {
	return &FallbackLLMClient{
		nodes:    nodes,
		template: NewTemplateEngine(),
		metrics:  metrics,
		logger:   log,
	}
}

// 甯﹂檷绾х殑 LLM 璋冪敤锛氫緷娆″皾璇曟瘡涓妭鐐癸紝鍏ㄩ儴澶辫触璧版ā鏉垮厹搴?
func (f *FallbackLLMClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	for i, node := range f.nodes {
		f.logger.Info("LLM 鑺傜偣璋冪敤寮€濮?, logger.Int("node_index", i))
		resp, err := node.ChatCompletion(ctx, req)
		if err == nil {
			if i > 0 {
				f.logger.Info("闄嶇骇鎴愬姛", logger.Int("node_index", i))
			}
			return resp, nil
		}
		f.metrics.IncLLMError()
		f.logger.Warn("鑺傜偣璋冪敤澶辫触锛屽皾璇曚笅涓€灞?,
			logger.Int("node_index", i),
			logger.Error(err))
	}

	f.logger.Error("鎵€鏈?LLM 鑺傜偣涓嶅彲鐢紝璧版ā鏉垮厹搴?)
	f.metrics.IncTemplateFallback()

	// 妯℃澘鍏滃簳
	content := f.template.GenerateContent(req.Messages)
	f.logger.Info("妯℃澘鍏滃簳鍥炲", logger.String("content_prefix", content[:min(len(content), 50)]))

	return &ChatResponse{
		ID:      "template-fallback",
		Created: 0,
		Choices: []pkgai.Choice{{
			Index: 0,
			Message: pkgai.Message{
				Role:    "assistant",
				Content: content,
			},
		}},
	}, nil
}

// 甯﹂檷绾х殑娴佸紡璋冪敤锛氫緷娆″皾璇曟瘡涓妭鐐癸紝鍏ㄩ儴澶辫触璧版ā鏉垮厹搴?
func (f *FallbackLLMClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	for i, node := range f.nodes {
		f.logger.Info("LLM 娴佸紡鑺傜偣璋冪敤寮€濮?, logger.Int("node_index", i))
		ch, err := node.ChatCompletionStream(ctx, req)
		if err == nil {
			if i > 0 {
				f.logger.Info("闄嶇骇鎴愬姛锛坰tream锛?, logger.Int("node_index", i))
			}
			return ch, nil
		}
		f.metrics.IncLLMError()
		f.logger.Warn("娴佸紡璋冪敤澶辫触锛屽皾璇曚笅涓€灞?,
			logger.Int("node_index", i),
			logger.Error(err))
	}

	f.logger.Error("鎵€鏈?LLM 鑺傜偣涓嶅彲鐢紝璧版ā鏉垮厹搴曪紙stream锛?)
	f.metrics.IncTemplateFallback()

	// 妯℃澘鍏滃簳锛氳繑鍥炲崟涓?chunk 鐨勬祦寮忓搷搴?
	content := f.template.GenerateContent(req.Messages)

	ch := make(chan ChatResponse, 1)
	finishReason := "stop"
	ch <- ChatResponse{
		ID:      "template-fallback",
		Created: 0,
		Choices: []pkgai.Choice{{
			Index:        0,
			FinishReason: &finishReason,
			Message: pkgai.Message{
				Role:    "assistant",
				Content: content,
			},
		}},
	}
	close(ch)
	return ch, nil
}

// 鍩轰簬鎰忓浘鐨勬ā鏉垮洖澶嶏紝鎵€鏈?LLM 鑺傜偣涓嶅彲鐢ㄦ椂鍏滃簳
// 鍏堣瘑鍒剰鍥撅紝鍐?鎰忓浘 -> 妯℃澘绛斿+缂撳瓨濂界殑闂鎺ㄨ崘
type TemplateEngine struct {
	templates map[domain.IntentType]string
}

func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		templates: map[domain.IntentType]string{
			domain.IntentReturn:         "閫€璐ф祦绋嬶細璇峰湪璁㈠崟璇︽儏椤电偣鍑汇€岀敵璇烽€€璐с€嶏紝閫夋嫨閫€璐у師鍥犲悗鎻愪氦銆?澶╂棤鐞嗙敱閫€璐у晢鍝佽鍦ㄧ鏀跺悗7澶╁唴鐢宠銆俓n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"閫€璐ц繍璐硅皝鎵挎媴\",\"閫€娆惧涔呭埌璐"]}",
			domain.IntentLogistics:      "鐗╂祦鏌ヨ锛氳鍦ㄣ€屾垜鐨勮鍗曘€嶉〉闈㈢偣鍑诲搴旇鍗曟煡鐪嬬墿娴佷俊鎭€傚鐗╂祦闀挎椂闂存湭鏇存柊锛屽缓璁仈绯讳汉宸ュ鏈嶃€俓n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"鐗╂祦澶氫箙鑳藉埌\",\"鍙互淇敼鏀惰揣鍦板潃鍚梊"]}",
			domain.IntentPayment:        "鏀粯闂锛氳妫€鏌ユ敮浠樻柟寮忔槸鍚︽甯革紝纭璐︽埛浣欓鍏呰冻銆傚浠嶆棤娉曟敮浠橈紝寤鸿鏇存崲鏀粯鏂瑰紡鎴栬仈绯讳汉宸ュ鏈嶃€俓n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"鏀寔鍝簺鏀粯鏂瑰紡\",\"浠樻鍚庡涔呭彂璐"]}",
			domain.IntentOrderInquiry:   "璁㈠崟鏌ヨ锛氳鍦ㄣ€屾垜鐨勮鍗曘€嶉〉闈㈡煡鐪嬭鍗曠姸鎬併€傚鏈夌枒闂紝寤鸿鑱旂郴浜哄伐瀹㈡湇銆俓n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"濡備綍鍙栨秷璁㈠崟\",\"璁㈠崟鐘舵€佽鏄嶾"]}",
			domain.IntentProductInquiry: "鍟嗗搧鍜ㄨ锛氬缓璁偍鏌ョ湅鍟嗗搧璇︽儏椤典簡瑙ｈ鏍煎弬鏁帮紝鎴栬仈绯讳汉宸ュ鏈嶈幏鍙栨洿澶氫俊鎭€俓n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"鏈夋病鏈変紭鎯犳椿鍔╘",\"鏀寔涓冨ぉ鏃犵悊鐢遍€€璐у悧\"]}",
			domain.IntentFAQ:            "鎰熻阿鎮ㄧ殑鍜ㄨ銆傚闇€甯姪锛岃鑱旂郴浜哄伐瀹㈡湇鑾峰彇璇︾粏瑙ｇ瓟銆俓n===META===\n{\"confidence\":0.6,\"emotion\":\"neutral\",\"suggested_questions\":[\"濡備綍鑱旂郴瀹㈡湇\",\"甯歌闂鍦ㄥ摢\"]}",
			domain.IntentComplaint:      "闈炲父鎶辨瓑缁欐偍甯︽潵涓嶄究锛屾偍鐨勯棶棰樻垜浠凡璁板綍銆傚缓璁偍鑱旂郴浜哄伐瀹㈡湇锛屾垜浠細灏藉揩涓烘偍澶勭悊銆俓n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"鎶曡瘔澶勭悊瑕佸涔匼",\"鍙互鐢宠璧斿伩鍚梊"]}",
			domain.IntentPromotion:      "娲诲姩鍜ㄨ锛氳鍏虫敞棣栭〉娲诲姩涓撳尯浜嗚В鏈€鏂颁紭鎯犱俊鎭€傚鏈夊叿浣撻棶棰橈紝寤鸿鑱旂郴浜哄伐瀹㈡湇銆俓n===META===\n{\"confidence\":0.6,\"emotion\":\"neutral\",\"suggested_questions\":[\"褰撳墠鏈変粈涔堜紭鎯燶",\"浼樻儬鍒告€庝箞棰嗗彇\"]}",
		},
	}
}

func (t *TemplateEngine) GenerateContent(messages []pkgai.Message) string {
	intent := t.detectIntent(messages)
	if tpl, ok := t.templates[intent]; ok {
		return tpl
	}
	return "鎶辨瓑锛岀郴缁熺箒蹇欙紝璇风◢鍚庨噸璇曟垨鑱旂郴浜哄伐瀹㈡湇銆俓n===META===\n{\"confidence\":0.3,\"emotion\":\"neutral\",\"suggested_questions\":[]}"
}

func (t *TemplateEngine) detectIntent(messages []pkgai.Message) domain.IntentType {
	var userMsg string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userMsg = messages[i].Content
			break
		}
	}
	if userMsg == "" {
		return domain.IntentUnknown
	}

	keywords := map[domain.IntentType][]string{
		domain.IntentReturn:         {"閫€璐?, "閫€娆?, "閫€鎹?, "涓冨ぉ鏃犵悊鐢?, "閫€鍥?},
		domain.IntentLogistics:      {"鐗╂祦", "蹇€?, "閰嶉€?, "鍙戣揣", "杩愯垂", "鍒颁簡鍚?},
		domain.IntentPayment:        {"鏀粯", "浠樻", "寰俊鏀粯", "鏀粯瀹?, "浠樹笉浜?},
		domain.IntentOrderInquiry:   {"璁㈠崟", "涓嬪崟", "鍙栨秷璁㈠崟", "璁㈠崟鐘舵€?},
		domain.IntentProductInquiry: {"鍟嗗搧", "浜у搧", "瑙勬牸", "灏虹爜", "棰滆壊", "搴撳瓨"},
		domain.IntentComplaint:      {"鎶曡瘔", "宸瘎", "涓嶆弧", "涓炬姤", "澶樊"},
		domain.IntentPromotion:      {"浼樻儬", "娲诲姩", "鎶樻墸", "淇冮攢", "鍒?, "婊″噺"},
		domain.IntentFAQ:            {"鎬庝箞", "濡備綍", "浠€涔堟槸", "甯姪", "鍦ㄥ摢"},
	}
	for intent, kws := range keywords {
		for _, kw := range kws {
			if strings.Contains(userMsg, kw) {
				return intent
			}
		}
	}
	return domain.IntentUnknown
}
