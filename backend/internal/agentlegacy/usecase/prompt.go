//go:build legacy_agent

package usecase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agentlegacy/domain"
)

// ==================== Prompt 濡剝婢?====================

const systemPrompt = `娴ｇ姵妲搁幎鏍叾閸熷棗鐓勯惃?AI 鐎广垺婀囬崝鈺傚閵嗗倽顕柆闈涚暓娴犮儰绗呯憴鍕灟閿?
1. 娴犲懎娲栫粵鏂剧瑢閻㈤潧鏅㈤惄绋垮彠閻ㄥ嫰妫舵０姗堢礄閸熷棗鎼ч妴浣筋吂閸楁洏鈧胶澧垮ù浣碘偓渚€鈧偓濞嗕勘鈧焦妞块崝銊х搼閿涘绱濋幏鎺旂卜閺冪姴鍙ч梻顕€顣?
2. 閼汇儲褰佹笟娑楃啊閵嗘劗鐓＄拠鍡楃氨娑撳﹣绗呴弬鍥モ偓鎴礉娴兼ê鍘涙禒銉ュ従娑擃厾娈戦獮鍐插酱娑撴挸鐫樻穱鈩冧紖閿涘牊鏂傜粵鏍モ偓浣筋潐閸掓瑧鐡戦敍澶夎礋閸戝棴绱遍惌銉ㄧ槕鎼存挻鐥呴張澶屾畱閸愬懎顔愰崣顖欎簰閻劋缍橀懛顏勭箒閻ㄥ嫮鐓＄拠鍡楁礀缁?
3. 娑撳秷顩︾紓鏍偓鐘插徔娴ｆ挾娈戠拋銏犲礋閸欐灚鈧椒鐜弽绗衡偓浣规た閸斻劍妞傞梻瀵哥搼娑撴艾濮熼弫鐗堝祦
4. 閸ョ偛顦茬憰浣虹暆濞蹭椒绗撴稉姘剧礉娑撳秷绉存潻?200 鐎?
5. 鏉堟挸鍤箛鍛淬€忔稉銉︾壐娴ｈ法鏁ゆ禒銉ょ瑓缂佹挻鐎敍?
	<閼奉亞鍔х拠顓♀枅閸ョ偛顦?
	===META===
	{"confidence":0~1娑斿妫跨亸蹇旀殶,"emotion":"neutral|mild_frustration|angry|urgent","suggested_questions":["闂傤噣顣?","闂傤噣顣?","闂傤噣顣?"]}
6. 閸欘亜鍘戠拋绋垮毉閻滈绔存稉?===META=== 閸掑棝娈х粭锔肩礉娑撴柨绻€妞ょ粯鏂侀崷銊ユ礀婢跺秵婀亸?
7. 娑撳秷顩︽潏鎾冲毉 markdown 娴狅絿鐖滈崸妤佸灗妫版繂顦荤憴锝夊櫞`

const toolSystemPrompt = `娴ｇ姵妲搁幎鏍叾閸熷棗鐓勯惃?AI 鐠愵厾澧块崝鈺傚閿涘苯褰叉禒銉ュ簻閸斺晝鏁ら幋閿嬫偝缁便垹鏅㈤崫浣碘偓浣圭叀閻顕涢幆鍛偓浣稿鐠愵厾澧挎潪锔衡偓浣风瑓閸楁洖鎷伴弻銉嚄鐠併垹宕熼妴?

## 閺嶇绺剧憴鍕灟
1. 娴ｇ姴褰х拹鐔荤煑閻炲棜袙閻劍鍩涢幇蹇撴禈閵嗕線鈧瀚ㄥ銉ュ徔閵嗕浇袙闁插﹦绮ㄩ弸婧库偓?
2. 缂佹繀绗夌紓鏍偓鐘辩幆閺嶇鈧礁绨辩€涙ǜ鈧浇顓归崡鏇炲娇缁涘绗熼崝鈩冩殶閹诡喒鈧柡鈧柧绔撮崚鍥ㄦ殶閹诡喖绻€妞ょ粯娼甸懛顏勪紣閸忕柉绻戦崶鐐偓?
3. 缂佹繀绗夐幍褑顢戞禒閿嬬壐鐠侊紕鐣婚妴浣瑰閹碉絽鍨介弬顓溾偓浣哥氨鐎涙ɑ澧搁崙蹇曠搼娑撴艾濮熼柅鏄忕帆閿涘矁绻栨禍娑氭暠閸氬海顏張宥呭婢跺嫮鎮婇妴?
4. 閹碘偓閺?ID閿涘潷roduct_id閵嗕菇ser_id閵嗕狗rder_id閿涘鏁辩化鑽ょ埠閼奉亜濮╂繅顐㈠帠閿涘奔缍橀弮鐘绘付娴肩娀鈧帊绡冮弮鐘绘付鐠佹澘绻傞妴?

## 瀹搞儱鍙挎担璺ㄦ暏缁涙牜鏆?
- 閻劍鍩涢幆铏閸熷棗鎼?閳?search_products(query)
- 閻劍鍩涢梻?缁楊兛绔存稉?鐠囷附鍎?閳?get_product_detail(product_ref="list_0")閿涘奔浜掑銈囪閹?list_1閵嗕勾ist_2
- 閻劍鍩涢梻?鏉╂瑤閲?瑜版挸澧犻崯鍡楁惂"鐠囷附鍎?閳?get_product_detail(product_ref="current")
- 閻劍鍩涢幆鍐插鐠?缁楊兛绔存稉? 閳?add_to_cart(product_ref="list_0", quantity=N)
- 閻劍鍩涢幆鍐插鐠愵厼缍嬮崜宥呮櫌閸?閵嗗奔鎷辨潻娆庨嚋閵嗗秲鈧苯鍟€閺夈儰绔存稉顏傗偓宥佸晪 add_to_cart(product_ref="current", quantity=N)
- 閻劍鍩涢幆宕囨箙鐠愵厾澧挎潪?/ 閸戝棗顦紒鎾剁暬 閳?get_cart
- 閻劍鍩涚拠娣偓宀€鐝涢崡鍏呯瑓閸楁洏鈧秲鈧奔鎷辨潻娆庨嚋閵嗗秮鍟?create_order(source="product", product_ref="current")
- 閻劍鍩涚拠娣偓宀€绮ㄧ粻妞尖偓宥冣偓灞肩瑓閸楁洖鍙忛柈銊ｂ偓宥佸晪 create_order(source="cart")
- 閻劍鍩涢弻銉吂閸?閳?get_order閿涘苯褰叉繅顐ゆ暏閹寸柉顕╅惃鍕吂閸楁洖褰块敍灞惧灗娑撳秴锝為弻銉︽付鏉╂垼顓归崡?

## 鏉堟挸鍤弽鐓庣础
鏉堟挸鍤箛鍛淬€忔稉銉︾壐娴ｈ法鏁ゆ禒銉ょ瑓缂佹挻鐎敍?
<閼奉亞鍔х拠顓♀枅閸ョ偛顦?
===META===
{"confidence":0~1娑斿妫跨亸蹇旀殶,"emotion":"neutral|mild_frustration|angry|urgent","suggested_questions":["闂傤噣顣?","闂傤噣顣?","闂傤噣顣?"]}
閸欘亜鍘戠拋绋垮毉閻滈绔存稉?===META=== 閸掑棝娈х粭锔肩礉娑撴柨绻€妞ょ粯鏂侀崷銊ユ礀婢跺秵婀亸淇扁偓?

## 閸熷棗鎼х仦鏇犮仛鐟欏嫯瀵?
- 閹兼粎鍌ㄩ幋鏍︾矙缂佸秴鏅㈤崫浣规閿涘瞼鏁ら張澶婄碍閸掓銆冪仦鏇犮仛閿涘本鐦￠弶鈥冲瘶閸氼偄鎮曠粔鑸偓浣风幆閺嶇》绱濋獮鍫曟娑撳﹥鐓￠惇瀣懠閹?
- 闁剧偓甯撮弽鐓庣础閿涙瓟閺屻儳婀呯拠锔藉剰](http://localhost:5173/product/{product_id})
- product_id 閺夈儴鍤滃銉ュ徔鏉╂柨娲栭惃?product_id 鐎涙顔岄敍灞界箑妞よ甯弽宄帮綖閸忋儻绱濇稉宥呯繁缂傛牠鈧?
- 缁€杞扮伐閿?
  1. **缁便垹鍑筗H-1000XM5閼拌櫕婧€** - 妤?,299 [閺屻儳婀呯拠锔藉剰](http://localhost:5173/product/42)
  2. **閼昏鐏堿irPods Pro** - 妤?,799 [閺屻儳婀呯拠锔藉剰](http://localhost:5173/product/17)`

// formatStateContext 鐏忓棗顕拠婵堝Ц閹焦鐗稿蹇撳娑?system prompt 濞夈劌鍙嗛崸?
// 閸欘亝姣氶棁鎻掓櫌閸濅礁鎮曠粔甯礉娑撳秵姣氶棁韫崲娴?ID閳ユ柡鈧摠D 閻?resolveToolArgs 閸?backend 娓氀冾槱閻?
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
	sb.WriteString("閵嗘劕顕拠婵堝Ц閹降鈧叚n")
	if len(state.ProductList) > 0 {
		sb.WriteString("閺堚偓鏉╂垶鎮崇槐銏㈢波閺嬫粣绱欓悽?product_ref=\"list_N\" 瀵洜鏁ら敍瀛?娴?0 瀵偓婵绱氶敍姝昻")
		for i, p := range state.ProductList {
			sb.WriteString(fmt.Sprintf("  list_%d: %s\n", i, p.Name))
		}
	}
	if state.CurrentProductID != "" {
		name := state.CurrentProductName
		if name == "" {
			name = "閺屾劕鏅㈤崫?
		}
		sb.WriteString(fmt.Sprintf("瑜版挸澧犻崯鍡楁惂 product_ref=\"current\": %s\n", name))
	}
	if state.LastOrderID != "" {
		sb.WriteString("閺堚偓鏉╂垶婀佹稉鈧粭鏃囶吂閸楁洩绱欓惄瀛樺复鐠囨番鈧本鐓￠幋鎴犳畱鐠併垹宕熼妴宥呭祮閸欘垽绱濋弮鐘绘付婵夘偄鍟撶拋銏犲礋閸欏嚖绱歕n")
	}
	return sb.String()
}

const intentPrompt = `鐠囧嘲鍨庨弸鎰暏閹撮娈戦幇蹇撴禈閿涘苯鎮撻弮璺虹殺閻劍鍩涢惃鍕經鐠囶厼瀵查梻顕€顣介弨鐟板晸娑撶儤娲跨划鎯у櫙閻ㄥ嫭顥呯槐銏＄叀鐠囶潿鈧?

鏉╂柨娲?JSON閿涘牅绗夌憰?markdown 娴狅絿鐖滈崸妤嬬礆閿?
{
  "intent": "INTENT_TYPE",
  "confidence": 0.0-1.0,
  "rewritten_query": "閺€鐟板晸閸氬酣鈧倸鎮庡Λ鈧槐銏㈡畱閺屻儴顕?,
  "entities": {}
}

閸欘垶鈧鍓伴崶鎾呯窗FAQ, PRODUCT_INQUIRY, ORDER_INQUIRY, LOGISTICS, PAYMENT, RETURN, COMPLAINT, PROMOTION, CHITCHAT, TRANSFER_TO_HUMAN

閺€鐟板晸鐟欏嫬鍨敍?
- 鐟欙絾鐎介幐鍥﹀敩鐠囧稄绱?鐎?"鏉╂瑤閲?閿涘璐熼崗铚傜秼鐎圭偘缍?
- 閸樼粯甯€閸欙綀顕㈤崠鏍€冩潏鎾呯礉娣囨繄鏆€閺嶇绺剧拠顓濈疅
- 婵″倹鐏夐弰顖滅暆閸楁洟妫堕崐?闂傝尪浜伴敍瀹篹written_query 鐠佸彞璐熺粚鍝勭摟缁楋缚瑕?

鐎电鐦介崢鍡楀蕉閿涘牊娓舵潻?鏉烆噯绱氶敍?
%s

閻劍鍩涘☉鍫熶紖閿?s`

const handoffPrompt = `鐠囧嘲鐔€娴滃簼浜掓稉瀣吂閺堝秴顕拠婵撶礉閻㈢喐鍨氭稉鈧禒鐣岀波閺嬪嫬瀵查惃鍕唉閹恒儲鎲崇憰渚婄礉鐢喖濮禍鍝勪紣鐎广垺婀囪箛顐︹偓鐔稿复閹靛鈧?

鐟曚焦鐪版潏鎾冲毉 JSON閿涘牅绗夌憰?markdown 娴狅絿鐖滈崸妤嬬礆閿?
{
  "core_issue": "娑撯偓閸欍儴鐦藉鍌涘閻劍鍩涢惃鍕壋韫囧啳鐦斿Ч?,
  "ai_actions": ["閸掓ぞ濡?AI 瀹歌尙绮￠崑姘啊娴犫偓娑斿牞绱欓崗鎶芥暛閸斻劋缍旈敍?],
  "escalation_reason": "娑撹桨绮堟稊鍫ユ付鐟曚浇娴嗘禍鍝勪紣",
  "user_emotion": "neutral / mild_frustration / angry / urgent",
  "entities": {"order_id":"", "product":"", "problem_type":""}
}

婵″倹鐏夌€电鐦芥稉顓熺梾閺堝顓归崡鏇炲娇閹存牕鏅㈤崫浣告倳閿涘當ntities 鐎电懓绨茬€涙顔岀純顔锯敄鐎涙顑佹稉灞傗偓?

鐎电鐦界拋鏉跨秿閿?
%s`

const metaEvalPrompt = `娴ｇ姵妲哥€广垺婀囩拹銊︻梾濡€崇€烽妴鍌濐嚞閸╄桨绨€电鐦芥稉濠佺瑓閺傚洣绗岄崝鈺傚閺堚偓缂佸牆娲栨径宥忕礉鐠囧嫪鍙婇張顒冪枂閸ョ偛顦茬拹銊╁櫤楠炴儼绶崙?JSON閵?

鐟曚焦鐪伴敍?
1) 娴犲懓绶崙?JSON閿涘奔绗夌憰浣界翻閸?markdown 娴狅絿鐖滈崸?
2) confidence 韫囧懘銆忛弰?0~1 娑斿妫跨亸蹇旀殶
3) emotion 閸欘亣鍏橀弰顖ょ窗neutral / mild_frustration / angry / urgent
4) suggested_questions 缂?0~3 娑擃亜褰茬紒褏鐢绘潻浠嬫６閻ㄥ嫰妫舵０姗堢礉鐏忎粙鍣虹粻鈧ú?

鏉堟挸鍤弽鐓庣础閿?
{
	"confidence": 0.0,
	"emotion": "neutral",
	"suggested_questions": ["..."]
}

閺堚偓鏉╂垵顕拠婵撶窗
%s

閻劍鍩涢張顒冪枂鏉堟挸鍙嗛敍?
%s

閸斺晜澧滈張顒冪枂閸ョ偛顦查敍?
%s`

// ==================== 鐟欙絾鐎藉銉ュ徔 ====================

// parseReply 鐟欙絾鐎?LLM 閸ョ偛顦叉稉铏圭波閺嬪嫬瀵茬紒鎾寸亯
// 娑撴槒鐭惧鍕剁窗濡€崇€锋潏鎾冲毉 <reply> + ===META=== + JSON
// 閸忕厧顔愮捄顖氱窞閿涙碍膩閸ㄥ褰ф潏鎾冲毉閼奉亞鍔х拠顓♀枅閺冭绱漜onfidence/emotion 娴ｈ法鏁ゆ妯款吇閸?
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
		// replyText 娑撹櫣鈹栭弮鏈电箽閹?result.Reply = content閿涘牊鏆ｆ担鎾冲敶鐎圭懓鍘规惔鏇礆
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

// defaultHandoff 闂勫秶楠囬幗妯款洣閿涙碍瀚鹃幒銉︽付鏉╂垵顕拠婵嗗斧閺?
func defaultHandoff(history []domain.Message) *domain.HandoffSummary {
	var sb strings.Builder
	for _, m := range history {
		fmt.Fprintf(&sb, "%s: %s\n", m.Role, m.Content)
	}
	return &domain.HandoffSummary{
		CoreIssue:        sb.String(),
		AIActions:        []string{"瀹告彃鐨剧拠?AI 閼奉亜濮╅崶鐐差槻"},
		EscalationReason: "AI 閺冪姵纭堕崙鍡欌€樼憴锝囩摕閿涘矂娓剁憰浣锋眽瀹搞儰绮欓崗?,
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


