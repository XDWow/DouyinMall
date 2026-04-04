package events

// ID瀹氫綅鏁版嵁锛孌irection 璇存槑璋佷负鍩哄噯锛宼ype 纭畾鎿嶄綔绫诲瀷锛?
// 1.浼犲叆闇€瑕佹洿鏂扮殑鍐呭锛屾秷璐硅€呮嬁鐫€鍐呭鐩存帴鏇存柊target锛岄棶棰樻槸杩欎釜鍐呭宸茬粡鏃т簡
// 2.浼犲叆id,娑堣垂鑰呭啀鍘籦ase涓煡鏁版嵁锛屾煡鍒扮殑鏁版嵁鏇存柊target,杩欐牱鏇磋兘淇濊瘉涓€鑷存€?
type InconsistentEvent struct {
	ID int64
	// 浠ヨ皝涓哄熀鍑?
	Direction string

	// 鏈変簺鏃跺€欙紝涓€浜涜娴嬶紝鎴栬€呬竴浜涚涓夋柟锛岄渶瑕佺煡閬擄紝鏄粈涔堝紩璧风殑涓嶄竴鑷?
	// 鍥犱负浠栬鍘?DEBUG
	// 杩欎釜鏄彲閫夌殑
	Type string
	// 浜嬩欢閲岄潰甯?base 鐨勬暟鎹?
	// 淇鏁版嵁鐢ㄨ繖閲岀殑鍘讳慨锛岃繖绉嶅仛娉曟槸涓嶈鐨勶紝鍥犱负鏈変弗閲嶇殑骞跺彂闂
	Columns map[string]any
}

const (
	// InconsistentEventTypeTargetMissing 鏍￠獙鐨勭洰鏍囨暟鎹紝缂轰簡杩欎竴鏉?
	InconsistentEventTypeTargetMissing = "target_missing"
	// InconsistentEventTypeNEQ 涓嶇浉绛?
	InconsistentEventTypeNEQ         = "neq"
	InconsistentEventTypeBaseMissing = "base_missing"
)


