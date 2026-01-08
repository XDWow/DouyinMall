package domain

type User struct {
	ID       int64
	UserName string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	Avatar   string `json:"avatar"`
}
