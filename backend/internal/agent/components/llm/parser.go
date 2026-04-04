package llm

import "encoding/json"

func DecodeJSON[T any](raw string) (T, error) {
	var out T
	err := json.Unmarshal([]byte(raw), &out)
	return out, err
}
