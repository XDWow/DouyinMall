package ginx

import (
	"net/http"
	"strconv"

	"github.com/XDWow/DouyinMall/backend/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// 鍙楀埗浜庢硾鍨嬶紝鎴戜滑杩欓噷鍙兘浣跨敤鍖呭彉閲忥紝鎴戞繁鎭剁棝缁濈殑鍖呭彉閲?
var log logger.LoggerV1 = logger.NewZapLogger(nil)

// 杩欎釜涓滆タ锛屾斁鍒颁綘浠殑 ginx 鎻掍欢搴撻噷闈㈠幓
// 鎶€鏈惈閲忎笉鏄緢楂橈紝浣嗘槸缁濆鏈夋妧宸?
// L 浣跨敤鍖呭彉閲?
// 鍖呭彉閲忓鑷存垜浠繖涓湴鏂圭殑浠ｇ爜闈炲父鍨冨溇
var vector *prometheus.CounterVec

// 杩欓噷鍒涘缓骞舵敞鍐?
// 鍦ㄥ叿浣撲唬鐮佷腑锛岃皟鐢?vector.WithLabelValues("200").Inc() 鏉ュ鍔犲搴旂姸鎬佺爜鐨勮鏁?
func InitCounter(opt prometheus.CounterOpts) {
	vector = prometheus.NewCounterVec(opt, []string{"code"})
	prometheus.MustRegister(vector)
	// 浣犺繕鍙互鑰冭檻浣跨敤 code, method, 鍛戒腑璺敱锛孒TTP 鐘舵€佺爜
}

func SetLogger(l logger.LoggerV1) {
	log = l
}

// WrapClaimsAndReq 濡傛灉鍋氭垚涓棿浠舵潵婧愬嚭鍘伙紝閭ｄ箞鐩存帴鑰﹀悎 UserClaims 涔熸槸涓嶅ソ鐨勩€?
func WrapClaimsAndReq[Req any](fn func(*gin.Context, Req, UserClaims) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req Req
		if err := ctx.Bind(&req); err != nil {
			log.Error("瑙ｆ瀽璇锋眰澶辫触", logger.Error(err))
			return
		}
		// 鍙互鐢ㄥ寘鍙橀噺鏉ラ厤缃紝杩樻槸閭ｅ彞璇濓紝鍥犱负娉涘瀷鐨勯檺鍒讹紝杩欓噷鍙兘鐢ㄥ寘鍙橀噺
		rawVal, ok := ctx.Get("user")
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			log.Error("鏃犳硶鑾峰緱 claims",
				logger.String("path", ctx.Request.URL.Path))
			return
		}
		// 娉ㄦ剰锛岃繖閲岃姹傛斁杩涘幓 ctx 鐨勪笉鑳芥槸*UserClaims锛岃繖鏄父瑙佺殑涓€涓敊璇?
		claims, ok := rawVal.(UserClaims)
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			log.Error("鏃犳硶鑾峰緱 claims",
				logger.String("path", ctx.Request.URL.Path))
			return
		}
		res, err := fn(ctx, req, claims)
		vector.WithLabelValues(strconv.Itoa(res.Code)).Inc()
		if err != nil {
			log.Error("鎵ц涓氬姟閫昏緫澶辫触",
				logger.Error(err))
		}
		ctx.JSON(http.StatusOK, res)
	}
}

// WrapReq 銆?
func WrapReq[Req any](fn func(*gin.Context, Req) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req Req
		if err := ctx.Bind(&req); err != nil {
			log.Error("瑙ｆ瀽璇锋眰澶辫触", logger.Error(err))
			return
		}
		res, err := fn(ctx, req)
		if err != nil {
			log.Error("鎵ц涓氬姟閫昏緫澶辫触",
				logger.Error(err))
		}
		vector.WithLabelValues(strconv.Itoa(res.Code)).Inc()
		ctx.JSON(http.StatusOK, res)
	}
}

func Wrap(fn func(*gin.Context) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := fn(ctx)
		if err != nil {
			log.Error("鎵ц涓氬姟閫昏緫澶辫触",
				logger.Error(err))
		}
		vector.WithLabelValues(strconv.Itoa(res.Code)).Inc()
		ctx.JSON(http.StatusOK, res)
	}
}

// WrapClaims 澶嶅埗绮樿创
func WrapClaims(fn func(*gin.Context, UserClaims) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 鍙互鐢ㄥ寘鍙橀噺鏉ラ厤缃紝杩樻槸閭ｅ彞璇濓紝鍥犱负娉涘瀷鐨勯檺鍒讹紝杩欓噷鍙兘鐢ㄥ寘鍙橀噺
		rawVal, ok := ctx.Get("user")
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			log.Error("鏃犳硶鑾峰緱 claims",
				logger.String("path", ctx.Request.URL.Path))
			return
		}
		// 娉ㄦ剰锛岃繖閲岃姹傛斁杩涘幓 ctx 鐨勪笉鑳芥槸*UserClaims锛岃繖鏄父瑙佺殑涓€涓敊璇?
		claims, ok := rawVal.(UserClaims)
		if !ok {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			log.Error("鏃犳硶鑾峰緱 claims",
				logger.String("path", ctx.Request.URL.Path))
			return
		}
		res, err := fn(ctx, claims)
		if err != nil {
			log.Error("鎵ц涓氬姟閫昏緫澶辫触",
				logger.Error(err))
		}
		vector.WithLabelValues(strconv.Itoa(res.Code)).Inc()
		ctx.JSON(http.StatusOK, res)
	}
}


