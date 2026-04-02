//go:build legacy_agent

package usecase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
)

// ==================== Prompt 妯℃澘 ====================

const systemPrompt = `浣犳槸鎶栭煶鍟嗗煄鐨?AI 瀹㈡湇鍔╂墜銆傝閬靛畧浠ヤ笅瑙勫垯锛?
1. 浠呭洖绛斾笌鐢靛晢鐩稿叧鐨勯棶棰橈紙鍟嗗搧銆佽鍗曘€佺墿娴併€侀€€娆俱€佹椿鍔ㄧ瓑锛夛紝鎷掔粷鏃犲叧闂
2. 鑻ユ彁渚涗簡銆愮煡璇嗗簱涓婁笅鏂囥€戯紝浼樺厛浠ュ叾涓殑骞冲彴涓撳睘淇℃伅锛堟斂绛栥€佽鍒欑瓑锛変负鍑嗭紱鐭ヨ瘑搴撴病鏈夌殑鍐呭鍙互鐢ㄤ綘鑷繁鐨勭煡璇嗗洖绛?
3. 涓嶈缂栭€犲叿浣撶殑璁㈠崟鍙枫€佷环鏍笺€佹椿鍔ㄦ椂闂寸瓑涓氬姟鏁版嵁
4. 鍥炲瑕佺畝娲佷笓涓氾紝涓嶈秴杩?200 瀛?
5. 杈撳嚭蹇呴』涓ユ牸浣跨敤浠ヤ笅缁撴瀯锛?
	<鑷劧璇█鍥炲>
	===META===
	{"confidence":0~1涔嬮棿灏忔暟,"emotion":"neutral|mild_frustration|angry|urgent","suggested_questions":["闂1","闂2","闂3"]}
6. 鍙厑璁稿嚭鐜颁竴涓?===META=== 鍒嗛殧绗︼紝涓斿繀椤绘斁鍦ㄥ洖澶嶆湯灏?
7. 涓嶈杈撳嚭 markdown 浠ｇ爜鍧楁垨棰濆瑙ｉ噴`

const toolSystemPrompt = `浣犳槸鎶栭煶鍟嗗煄鐨?AI 璐墿鍔╂墜锛屽彲浠ュ府鍔╃敤鎴锋悳绱㈠晢鍝併€佹煡鐪嬭鎯呫€佸姞璐墿杞︺€佷笅鍗曞拰鏌ヨ璁㈠崟銆?

## 鏍稿績瑙勫垯
1. 浣犲彧璐熻矗鐞嗚В鐢ㄦ埛鎰忓浘銆侀€夋嫨宸ュ叿銆佽В閲婄粨鏋溿€?
2. 缁濅笉缂栭€犱环鏍笺€佸簱瀛樸€佽鍗曞彿绛変笟鍔℃暟鎹€斺€斾竴鍒囨暟鎹繀椤绘潵鑷伐鍏疯繑鍥炪€?
3. 缁濅笉鎵ц浠锋牸璁＄畻銆佹姌鎵ｅ垽鏂€佸簱瀛樻墸鍑忕瓑涓氬姟閫昏緫锛岃繖浜涚敱鍚庣鏈嶅姟澶勭悊銆?
4. 鎵€鏈?ID锛坧roduct_id銆乽ser_id銆乷rder_id锛夌敱绯荤粺鑷姩濉厖锛屼綘鏃犻渶浼犻€掍篃鏃犻渶璁板繂銆?

## 宸ュ叿浣跨敤绛栫暐
- 鐢ㄦ埛鎯虫壘鍟嗗搧 鈫?search_products(query)
- 鐢ㄦ埛闂?绗竴涓?璇︽儏 鈫?get_product_detail(product_ref="list_0")锛屼互姝ょ被鎺?list_1銆乴ist_2
- 鐢ㄦ埛闂?杩欎釜/褰撳墠鍟嗗搧"璇︽儏 鈫?get_product_detail(product_ref="current")
- 鐢ㄦ埛鎯冲姞璐?绗竴涓? 鈫?add_to_cart(product_ref="list_0", quantity=N)
- 鐢ㄦ埛鎯冲姞璐綋鍓嶅晢鍝?銆屼拱杩欎釜銆嶃€屽啀鏉ヤ竴涓€嶁啋 add_to_cart(product_ref="current", quantity=N)
- 鐢ㄦ埛鎯崇湅璐墿杞?/ 鍑嗗缁撶畻 鈫?get_cart
- 鐢ㄦ埛璇淬€岀珛鍗充笅鍗曘€嶃€屼拱杩欎釜銆嶁啋 create_order(source="product", product_ref="current")
- 鐢ㄦ埛璇淬€岀粨绠椼€嶃€屼笅鍗曞叏閮ㄣ€嶁啋 create_order(source="cart")
- 鐢ㄦ埛鏌ヨ鍗?鈫?get_order锛屽彲濉敤鎴疯鐨勮鍗曞彿锛屾垨涓嶅～鏌ユ渶杩戣鍗?

## 杈撳嚭鏍煎紡
杈撳嚭蹇呴』涓ユ牸浣跨敤浠ヤ笅缁撴瀯锛?
<鑷劧璇█鍥炲>
===META===
{"confidence":0~1涔嬮棿灏忔暟,"emotion":"neutral|mild_frustration|angry|urgent","suggested_questions":["闂1","闂2","闂3"]}
鍙厑璁稿嚭鐜颁竴涓?===META=== 鍒嗛殧绗︼紝涓斿繀椤绘斁鍦ㄥ洖澶嶆湯灏俱€?

## 鍟嗗搧灞曠ず瑙勮寖
- 鎼滅储鎴栦粙缁嶅晢鍝佹椂锛岀敤鏈夊簭鍒楄〃灞曠ず锛屾瘡鏉″寘鍚悕绉般€佷环鏍硷紝骞堕檮涓婃煡鐪嬮摼鎺?
- 閾炬帴鏍煎紡锛歔鏌ョ湅璇︽儏](http://localhost:5173/product/{product_id})
- product_id 鏉ヨ嚜宸ュ叿杩斿洖鐨?product_id 瀛楁锛屽繀椤诲師鏍峰～鍏ワ紝涓嶅緱缂栭€?
- 绀轰緥锛?
  1. **绱㈠凹WH-1000XM5鑰虫満** - 楼2,299 [鏌ョ湅璇︽儏](http://localhost:5173/product/42)
  2. **鑻规灉AirPods Pro** - 楼1,799 [鏌ョ湅璇︽儏](http://localhost:5173/product/17)`

// formatStateContext 灏嗗璇濈姸鎬佹牸寮忓寲涓?system prompt 娉ㄥ叆鍧?
// 鍙毚闇插晢鍝佸悕绉帮紝涓嶆毚闇蹭换浣?ID鈥斺€擨D 鐢?resolveToolArgs 鍦?backend 渚у鐞?
func formatStateContext(state *domain.EntityMemory) string {
	if state == nil {
		return ""
	}
	hasEntities := len(state.ProductList) > 0 ||
		state.CurrentProductID != "" ||
		state.LastOrderID != ""
	if !hasEntities {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("銆愬璇濈姸鎬併€慭n")
	if len(state.ProductList) > 0 {
		sb.WriteString("鏈€杩戞悳绱㈢粨鏋滐紙鐢?product_ref=\"list_N\" 寮曠敤锛孨 浠?0 寮€濮嬶級锛歕n")
		for i, p := range state.ProductList {
			sb.WriteString(fmt.Sprintf("  list_%d: %s\n", i, p.Name))
		}
	}
	if state.CurrentProductID != "" {
		name := state.CurrentProductName
		if name == "" {
			name = "鏌愬晢鍝?
		}
		sb.WriteString(fmt.Sprintf("褰撳墠鍟嗗搧 product_ref=\"current\": %s\n", name))
	}
	if state.LastOrderID != "" {
		sb.WriteString("鏈€杩戞湁涓€绗旇鍗曪紙鐩存帴璇淬€屾煡鎴戠殑璁㈠崟銆嶅嵆鍙紝鏃犻渶濉啓璁㈠崟鍙凤級\n")
	}
	return sb.String()
}

const intentPrompt = `璇峰垎鏋愮敤鎴风殑鎰忓浘锛屽悓鏃跺皢鐢ㄦ埛鐨勫彛璇寲闂鏀瑰啓涓烘洿绮惧噯鐨勬绱㈡煡璇€?

杩斿洖 JSON锛堜笉瑕?markdown 浠ｇ爜鍧楋級锛?
{
  "intent": "INTENT_TYPE",
  "confidence": 0.0-1.0,
  "rewritten_query": "鏀瑰啓鍚庨€傚悎妫€绱㈢殑鏌ヨ",
  "entities": {}
}

鍙€夋剰鍥撅細FAQ, PRODUCT_INQUIRY, ORDER_INQUIRY, LOGISTICS, PAYMENT, RETURN, COMPLAINT, PROMOTION, CHITCHAT, TRANSFER_TO_HUMAN

鏀瑰啓瑙勫垯锛?
- 瑙ｆ瀽鎸囦唬璇嶏紙"瀹?"杩欎釜"锛変负鍏蜂綋瀹炰綋
- 鍘绘帀鍙ｈ鍖栬〃杈撅紝淇濈暀鏍稿績璇箟
- 濡傛灉鏄畝鍗曢棶鍊?闂茶亰锛宺ewritten_query 璁句负绌哄瓧绗︿覆

瀵硅瘽鍘嗗彶锛堟渶杩?杞級锛?
%s

鐢ㄦ埛娑堟伅锛?s`

const handoffPrompt = `璇峰熀浜庝互涓嬪鏈嶅璇濓紝鐢熸垚涓€浠界粨鏋勫寲鐨勪氦鎺ユ憳瑕侊紝甯姪浜哄伐瀹㈡湇蹇€熸帴鎵嬨€?

瑕佹眰杈撳嚭 JSON锛堜笉瑕?markdown 浠ｇ爜鍧楋級锛?
{
  "core_issue": "涓€鍙ヨ瘽姒傛嫭鐢ㄦ埛鐨勬牳蹇冭瘔姹?,
  "ai_actions": ["鍒椾妇 AI 宸茬粡鍋氫簡浠€涔堬紙鍏抽敭鍔ㄤ綔锛?],
  "escalation_reason": "涓轰粈涔堥渶瑕佽浆浜哄伐",
  "user_emotion": "neutral / mild_frustration / angry / urgent",
  "entities": {"order_id":"", "product":"", "problem_type":""}
}

濡傛灉瀵硅瘽涓病鏈夎鍗曞彿鎴栧晢鍝佸悕锛宔ntities 瀵瑰簲瀛楁缃┖瀛楃涓层€?

瀵硅瘽璁板綍锛?
%s`

const metaEvalPrompt = `浣犳槸瀹㈡湇璐ㄦ妯″瀷銆傝鍩轰簬瀵硅瘽涓婁笅鏂囦笌鍔╂墜鏈€缁堝洖澶嶏紝璇勪及鏈疆鍥炲璐ㄩ噺骞惰緭鍑?JSON銆?

瑕佹眰锛?
1) 浠呰緭鍑?JSON锛屼笉瑕佽緭鍑?markdown 浠ｇ爜鍧?
2) confidence 蹇呴』鏄?0~1 涔嬮棿灏忔暟
3) emotion 鍙兘鏄細neutral / mild_frustration / angry / urgent
4) suggested_questions 缁?0~3 涓彲缁х画杩介棶鐨勯棶棰橈紝灏介噺绠€娲?

杈撳嚭鏍煎紡锛?
{
	"confidence": 0.0,
	"emotion": "neutral",
	"suggested_questions": ["..."]
}

鏈€杩戝璇濓細
%s

鐢ㄦ埛鏈疆杈撳叆锛?
%s

鍔╂墜鏈疆鍥炲锛?
%s`

// ==================== 瑙ｆ瀽宸ュ叿 ====================

// parseReply 瑙ｆ瀽 LLM 鍥炲涓虹粨鏋勫寲缁撴灉
// 涓昏矾寰勶細妯″瀷杈撳嚭 <reply> + ===META=== + JSON
// 鍏煎璺緞锛氭ā鍨嬪彧杈撳嚭鑷劧璇█鏃讹紝confidence/emotion 浣跨敤榛樿鍊?
func parseReply(content string) *domain.GenerationResult {
	content = strings.TrimSpace(content)
	result := &domain.GenerationResult{Reply: content, Confidence: 0.75, Emotion: "neutral", MetaSource: "default"}

	const sep = "===META==="
	if idx := strings.Index(content, sep); idx >= 0 {
		replyText := strings.TrimSpace(content[:idx])
		metaStr := cleanJSON(strings.TrimSpace(content[idx+len(sep):]))
		var meta struct {
			Confidence         float32  `json:"confidence"`
			Emotion            string   `json:"emotion"`
			SuggestedQuestions []string `json:"suggested_questions"`
		}
		if err := json.Unmarshal([]byte(metaStr), &meta); err == nil {
			confOK := meta.Confidence >= 0 && meta.Confidence <= 1
			emotion := strings.TrimSpace(meta.Emotion)
			emotionOK := isValidEmotion(emotion)
			if confOK {
				result.Confidence = meta.Confidence
			}
			if emotionOK {
				result.Emotion = emotion
			}
			result.Suggested = meta.SuggestedQuestions
			if confOK && emotionOK {
				result.MetaSource = "inline"
			}
		}
		if replyText != "" {
			result.Reply = replyText
		}
		// replyText 涓虹┖鏃朵繚鎸?result.Reply = content锛堟暣浣撳唴瀹瑰厹搴曪級
	}
	return result
}

func isValidEmotion(v string) bool {
	switch strings.TrimSpace(v) {
	case "neutral", "mild_frustration", "angry", "urgent":
		return true
	default:
		return false
	}
}

// defaultHandoff 闄嶇骇鎽樿锛氭嫾鎺ユ渶杩戝璇濆師鏂?
func defaultHandoff(history []domain.Message) *domain.HandoffSummary {
	var sb strings.Builder
	for _, m := range history {
		fmt.Fprintf(&sb, "%s: %s\n", m.Role, m.Content)
	}
	return &domain.HandoffSummary{
		CoreIssue:        sb.String(),
		AIActions:        []string{"宸插皾璇?AI 鑷姩鍥炲"},
		EscalationReason: "AI 鏃犳硶鍑嗙‘瑙ｇ瓟锛岄渶瑕佷汉宸ヤ粙鍏?,
		UserEmotion:      "unknown",
		Entities:         map[string]string{},
	}
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func mapIntent(s string) domain.IntentType {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "FAQ":
		return domain.IntentFAQ
	case "PRODUCT_INQUIRY":
		return domain.IntentProductInquiry
	case "ORDER_INQUIRY":
		return domain.IntentOrderInquiry
	case "LOGISTICS":
		return domain.IntentLogistics
	case "PAYMENT":
		return domain.IntentPayment
	case "RETURN":
		return domain.IntentReturn
	case "COMPLAINT":
		return domain.IntentComplaint
	case "PROMOTION":
		return domain.IntentPromotion
	case "CHITCHAT":
		return domain.IntentChitchat
	case "TRANSFER_TO_HUMAN":
		return domain.IntentTransferToHuman
	default:
		return domain.IntentUnknown
	}
}
