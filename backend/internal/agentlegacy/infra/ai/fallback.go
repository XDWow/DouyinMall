//go:build legacy_agent

package ai

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
	pkgai "github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// 閺堚偓鐏忓繐瀵查幐鍥ㄧ垼閹恒儱褰涢敍宀€鏁?usecase.PipelineMetrics 闂呮劕绱＄€圭偟骞?
type FallbackMetrics interface {
	IncTemplateFallback()
	IncLLMError()
}

// FallbackLLMClient 闂勫秶楠囩憗鍛淬偘閸ｎ煉绱伴崣顏囩鐠?閼哄倻鍋婢惰精瑙﹂埆鎺曠槸閼哄倻鍋閳帟鐦Ο鈩冩緲"鏉╂瑤绔存禒鏈电皑
// 濮ｅ繋閲滈懞鍌滃仯瀹歌尙鏁?ResilientClient 鐟佸懘銈伴敍鍫ユ濞?+ 閻旀梹鏌?+ 鐡掑懏妞傞敍澶涚礉FallbackLLMClient 娑撳秵鍔呴惌銉ュ徔娴ｆ挸甯崶?
type FallbackLLMClient struct {
	nodes    []CSLLMClient // 鐟佸懘銈?ResilientClient
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

// 鐢箓妾风痪褏娈?LLM 鐠嬪啰鏁ら敍姘贩濞嗏€崇毦鐠囨洘鐦℃稉顏囧Ν閻愮櫢绱濋崗銊╁劥婢惰精瑙︾挧鐗埬侀弶鍨幑鎼?
func (f *FallbackLLMClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	for i, node := range f.nodes {
		f.logger.Info("LLM 閼哄倻鍋ｇ拫鍐暏瀵偓婵?, logger.Int("node_index", i))
		resp, err := node.ChatCompletion(ctx, req)
		if err == nil {
			if i > 0 {
				f.logger.Info("闂勫秶楠囬幋鎰", logger.Int("node_index", i))
			}
			return resp, nil
		}
		f.metrics.IncLLMError()
		f.logger.Warn("閼哄倻鍋ｇ拫鍐暏婢惰精瑙﹂敍灞界毦鐠囨洑绗呮稉鈧仦?,
			logger.Int("node_index", i),
			logger.Error(err))
	}

	f.logger.Error("閹碘偓閺?LLM 閼哄倻鍋ｆ稉宥呭讲閻㈩煉绱濈挧鐗埬侀弶鍨幑鎼?)
	f.metrics.IncTemplateFallback()

	// 濡剝婢橀崗婊冪俺
	content := f.template.GenerateContent(req.Messages)
	f.logger.Info("濡剝婢橀崗婊冪俺閸ョ偛顦?, logger.String("content_prefix", content[:min(len(content), 50)]))

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

// 鐢箓妾风痪褏娈戝ù浣哥础鐠嬪啰鏁ら敍姘贩濞嗏€崇毦鐠囨洘鐦℃稉顏囧Ν閻愮櫢绱濋崗銊╁劥婢惰精瑙︾挧鐗埬侀弶鍨幑鎼?
func (f *FallbackLLMClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	for i, node := range f.nodes {
		f.logger.Info("LLM 濞翠礁绱￠懞鍌滃仯鐠嬪啰鏁ゅ鈧慨?, logger.Int("node_index", i))
		ch, err := node.ChatCompletionStream(ctx, req)
		if err == nil {
			if i > 0 {
				f.logger.Info("闂勫秶楠囬幋鎰閿涘澃tream閿?, logger.Int("node_index", i))
			}
			return ch, nil
		}
		f.metrics.IncLLMError()
		f.logger.Warn("濞翠礁绱＄拫鍐暏婢惰精瑙﹂敍灞界毦鐠囨洑绗呮稉鈧仦?,
			logger.Int("node_index", i),
			logger.Error(err))
	}

	f.logger.Error("閹碘偓閺?LLM 閼哄倻鍋ｆ稉宥呭讲閻㈩煉绱濈挧鐗埬侀弶鍨幑鎼存洩绱檚tream閿?)
	f.metrics.IncTemplateFallback()

	// 濡剝婢橀崗婊冪俺閿涙俺绻戦崶鐐插礋娑?chunk 閻ㄥ嫭绁﹀蹇撴惙鎼?
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

// 閸╄桨绨幇蹇撴禈閻ㄥ嫭膩閺夊灝娲栨径宥忕礉閹碘偓閺?LLM 閼哄倻鍋ｆ稉宥呭讲閻劍妞傞崗婊冪俺
// 閸忓牐鐦戦崚顐ｅ壈閸ユ拝绱濋崘?閹板繐娴?-> 濡剝婢樼粵鏂款槻+缂傛挸鐡ㄦ總鐣屾畱闂傤噣顣介幒銊ㄥ礃
type TemplateEngine struct {
	templates map[domain.IntentType]string
}

func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		templates: map[domain.IntentType]string{
			domain.IntentReturn:         "闁偓鐠愌勭ウ缁嬪绱扮拠宄版躬鐠併垹宕熺拠锔藉剰妞ょ數鍋ｉ崙姹団偓宀€鏁电拠鐑解偓鈧拹褋鈧稄绱濋柅澶嬪闁偓鐠愌冨斧閸ョ姴鎮楅幓鎰唉閵?婢垛晜妫ら悶鍡欐暠闁偓鐠愌冩櫌閸濅浇顕崷銊ь劮閺€璺烘倵7婢垛晛鍞撮悽瀹狀嚞閵嗕繐n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"闁偓鐠愌嗙箥鐠愮鐨濋幍鎸庡\",\"闁偓濞嗘儳顦挎稊鍛煂鐠愵泜"]}",
			domain.IntentLogistics:      "閻椻晜绁﹂弻銉嚄閿涙俺顕崷銊ｂ偓灞惧灉閻ㄥ嫯顓归崡鏇樷偓宥夈€夐棃銏㈠仯閸戣顕惔鏃囶吂閸楁洘鐓￠惇瀣⒖濞翠椒淇婇幁顖樷偓鍌氼洤閻椻晜绁﹂梹鎸庢闂傚瓨婀弴瀛樻煀閿涘苯缂撶拋顔夸粓缁姹夊銉ヮ吂閺堝秲鈧繐n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"閻椻晜绁︽径姘畽閼宠棄鍩孿",\"閸欘垯浜掓穱顔芥暭閺€鎯版彛閸︽澘娼冮崥姊?]}",
			domain.IntentPayment:        "閺€顖欑帛闂傤噣顣介敍姘愁嚞濡偓閺屻儲鏁禒妯绘煙瀵繑妲搁崥锔筋劀鐢潻绱濈涵顔款吇鐠愶附鍩涙担娆擃杺閸忓懓鍐婚妴鍌氼洤娴犲秵妫ゅ▔鏇熸暜娴犳﹫绱濆楦款唴閺囧瓨宕查弨顖欑帛閺傜懓绱￠幋鏍粓缁姹夊銉ヮ吂閺堝秲鈧繐n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"閺€顖涘瘮閸濐亙绨洪弨顖欑帛閺傜懓绱",\"娴犳ɑ顑欓崥搴☆樋娑斿懎褰傜拹顪?]}",
			domain.IntentOrderInquiry:   "鐠併垹宕熼弻銉嚄閿涙俺顕崷銊ｂ偓灞惧灉閻ㄥ嫯顓归崡鏇樷偓宥夈€夐棃銏＄叀閻顓归崡鏇犲Ц閹降鈧倸顩ч張澶屾瀿闂傤噯绱濆楦款唴閼辨梻閮存禍鍝勪紣鐎广垺婀囬妴淇搉===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"婵″倷缍嶉崣鏍ㄧХ鐠併垹宕焅",\"鐠併垹宕熼悩鑸碘偓浣筋嚛閺勫毒"]}",
			domain.IntentProductInquiry: "閸熷棗鎼ч崪銊嚄閿涙艾缂撶拋顔藉亶閺屻儳婀呴崯鍡楁惂鐠囷附鍎忔い鍏哥啊鐟欙綀顫夐弽鐓庡棘閺佸府绱濋幋鏍粓缁姹夊銉ヮ吂閺堝秷骞忛崣鏍ㄦ纯婢舵矮淇婇幁顖樷偓淇搉===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"閺堝鐥呴張澶夌喘閹姵妞块崝鈺?,\"閺€顖涘瘮娑撳啫銇夐弮鐘垫倞閻㈤亶鈧偓鐠愌冩偋\"]}",
			domain.IntentFAQ:            "閹扮喕闃块幃銊ф畱閸溿劏顕楅妴鍌氼洤闂団偓鐢喖濮敍宀冾嚞閼辨梻閮存禍鍝勪紣鐎广垺婀囬懢宄板絿鐠囷妇绮忕憴锝囩摕閵嗕繐n===META===\n{\"confidence\":0.6,\"emotion\":\"neutral\",\"suggested_questions\":[\"婵″倷缍嶉懕鏃傞兇鐎广垺婀嘰",\"鐢瓕顫嗛梻顕€顣介崷銊ユ憿\"]}",
			domain.IntentComplaint:      "闂堢偛鐖堕幎杈ㄧ搼缂佹瑦鍋嶇敮锔芥降娑撳秳绌堕敍灞惧亶閻ㄥ嫰妫舵０妯诲灉娴狀剙鍑＄拋鏉跨秿閵嗗倸缂撶拋顔藉亶閼辨梻閮存禍鍝勪紣鐎广垺婀囬敍灞惧灉娴狀兛绱扮亸钘夋彥娑撶儤鍋嶆径鍕倞閵嗕繐n===META===\n{\"confidence\":0.7,\"emotion\":\"neutral\",\"suggested_questions\":[\"閹舵洝鐦旀径鍕倞鐟曚礁顦挎稊鍖?,\"閸欘垯浜掗悽瀹狀嚞鐠ф柨浼╅崥姊?]}",
			domain.IntentPromotion:      "濞茶濮╅崪銊嚄閿涙俺顕崗铏暈妫ｆ牠銆夊ú璇插З娑撴挸灏禍鍡毿掗張鈧弬棰佺喘閹姳淇婇幁顖樷偓鍌氼洤閺堝鍙挎担鎾绘６妫版﹫绱濆楦款唴閼辨梻閮存禍鍝勪紣鐎广垺婀囬妴淇搉===META===\n{\"confidence\":0.6,\"emotion\":\"neutral\",\"suggested_questions\":[\"瑜版挸澧犻張澶夌矆娑斿牅绱幆鐕?,\"娴兼ɑ鍎崚鍛娾偓搴濈疄妫板棗褰嘰"]}",
		},
	}
}

func (t *TemplateEngine) GenerateContent(messages []pkgai.Message) string {
	intent := t.detectIntent(messages)
	if tpl, ok := t.templates[intent]; ok {
		return tpl
	}
	return "閹惰鲸鐡戦敍宀€閮寸紒鐔虹畳韫囨瑱绱濈拠椋庘棦閸氬酣鍣哥拠鏇熷灗閼辨梻閮存禍鍝勪紣鐎广垺婀囬妴淇搉===META===\n{\"confidence\":0.3,\"emotion\":\"neutral\",\"suggested_questions\":[]}"
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
		domain.IntentReturn:         {"闁偓鐠?, "闁偓濞?, "闁偓閹?, "娑撳啫銇夐弮鐘垫倞閻?, "闁偓閸?},
		domain.IntentLogistics:      {"閻椻晜绁?, "韫囶偊鈧?, "闁板秹鈧?, "閸欐垼鎻?, "鏉╂劘鍨?, "閸掗绨￠崥?},
		domain.IntentPayment:        {"閺€顖欑帛", "娴犳ɑ顑?, "瀵邦喕淇婇弨顖欑帛", "閺€顖欑帛鐎?, "娴犳ü绗夋禍?},
		domain.IntentOrderInquiry:   {"鐠併垹宕?, "娑撳宕?, "閸欐牗绉风拋銏犲礋", "鐠併垹宕熼悩鑸碘偓?},
		domain.IntentProductInquiry: {"閸熷棗鎼?, "娴溠冩惂", "鐟欏嫭鐗?, "鐏忚櫣鐖?, "妫版粏澹?, "鎼存挸鐡?},
		domain.IntentComplaint:      {"閹舵洝鐦?, "瀹割喛鐦?, "娑撳秵寮?, "娑撶偓濮?, "婢额亜妯?},
		domain.IntentPromotion:      {"娴兼ɑ鍎?, "濞茶濮?, "閹舵ɑ澧?, "娣囧啴鏀?, "閸?, "濠娾€冲櫤"},
		domain.IntentFAQ:            {"閹簼绠?, "婵″倷缍?, "娴犫偓娑斿牊妲?, "鐢喖濮?, "閸︺劌鎽?},
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


