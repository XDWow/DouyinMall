package handler

import (
	"net/http"

	"github.com/XDWow/DouyinMall/backend/pkg/ginx"
	"github.com/XDWow/DouyinMall/backend/pkg/jwtx"
	userv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/user/v1"
	"github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/user/v1/userservice"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userClient userservice.Client
	jwt        *jwtx.JWTManager
}

func NewAuthHandler(userClient userservice.Client, jwt *jwtx.JWTManager) *AuthHandler {
	return &AuthHandler{userClient: userClient, jwt: jwt}
}

func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/login", h.Login)
	rg.POST("/signup", h.Signup)
	rg.POST("/refresh", h.Refresh)
}

type loginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "请求参数错误: " + err.Error()})
		return
	}

	resp, err := h.userClient.Login(c.Request.Context(), &userv1.LoginReq{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, ginx.Result{Code: 4, Msg: "邮箱或密码错误"})
		return
	}

	h.issueTokens(c, resp.GetUserId())
}

type signupReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

func (h *AuthHandler) Signup(c *gin.Context) {
	var req signupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "请求参数错误: " + err.Error()})
		return
	}

	resp, err := h.userClient.Signup(c.Request.Context(), &userv1.SignupReq{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "注册失败: " + err.Error()})
		return
	}

	h.issueTokens(c, resp.GetUserId())
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.Result{Code: 4, Msg: "请求参数错误"})
		return
	}

	access, refresh, err := h.jwt.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ginx.Result{Code: 4, Msg: "refresh token 无效或已过期，请重新登录"})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"access_token":  access,
		"refresh_token": refresh,
	}})
}

func (h *AuthHandler) issueTokens(c *gin.Context, userID int64) {
	access, refresh, err := h.jwt.GenerateTokenPair(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{Code: 5, Msg: "生成 token 失败"})
		return
	}

	c.JSON(http.StatusOK, ginx.Result{Code: 0, Data: gin.H{
		"access_token":  access,
		"refresh_token": refresh,
		"user_id":       userID,
	}})
}
