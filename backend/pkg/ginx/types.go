package ginx

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Server struct {
	*gin.Engine
	Addr string
}

func (s *Server) Start() error {
	return s.Engine.Run(s.Addr)
}

// Result 浣犲彲浠ラ€氳繃鍦?Result 閲岄潰瀹氫箟鏇村姞澶氱殑瀛楁锛屾潵閰嶅悎 Wrap 鏂规硶
type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type UserClaims struct {
	Id        int64
	UserAgent string
	Ssid      string
	jwt.RegisteredClaims
}


