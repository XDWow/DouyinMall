package canalx

// Message 鍙互鏍规嵁闇€瑕佹妸鍏跺畠瀛楁涔熷姞鍏ヨ繘鏉ャ€?
type Message[T any] struct {
	Data     []T    `json:"data"`
	Database string `json:"database"`
	Table    string `json:"table"`
	Type     string `json:"type"`
}


