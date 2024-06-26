package router

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"
	helper "you-helper"
	"you-helper/logger"
)

func Bind(port int, p string) {
	gin.SetMode(gin.ReleaseMode)
	route := gin.Default()

	route.Use(cros)
	route.Use(panicError)
	route.Use(proxies(p))
	route.GET("/register", register)
	route.GET("/clearance", clearance)
	route.Static("tmp", "tmp")
	addr := ":" + strconv.Itoa(port)
	logger.Infof("server start by http://0.0.0.0%s", addr)
	if err := route.Run(addr); err != nil {
		logger.Fatal(err)
	}
}

func proxies(p string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("proxies", p)
	}
}

func clearance(ctx *gin.Context) {
	timeout, cancel := context.WithTimeout(ctx.Request.Context(), 180*time.Second)
	defer cancel()

	cookies, lang, err := helper.Clearance(timeout, ctx.GetString("proxies"))
	if err != nil {
		ctx.JSON(500, gin.H{
			"ok":  false,
			"msg": err.Error(),
		})
		return
	}

	ctx.JSON(200, gin.H{
		"ok":  true,
		"msg": "success",
		"data": gin.H{
			"lang":      lang,
			"userAgent": helper.UserAgent,
			"cookie":    cookies,
		},
	})
}

func register(ctx *gin.Context) {
	timeout, cancel := context.WithTimeout(ctx.Request.Context(), 120*time.Second)
	defer cancel()

	authorization := ctx.Request.Header.Get("Authorization")
	if authorization != "114514" {
		ctx.JSON(200, gin.H{
			"ok":  true,
			"msg": "success",
		})
		return
	}

	cookies, err := helper.Register(timeout, ctx.GetString("proxies"))
	if err != nil {
		ctx.JSON(500, gin.H{
			"ok":  false,
			"msg": err.Error(),
		})
		return
	}
	ctx.JSON(200, gin.H{
		"ok":   true,
		"msg":  "success",
		"data": cookies,
	})
}

func cros(context *gin.Context) {
	method := context.Request.Method
	context.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	context.Header("Access-Control-Allow-Origin", "*") // 设置允许访问所有域
	context.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE,UPDATE")
	context.Header("Access-Control-Allow-Headers", "*")
	context.Header("Access-Control-Expose-Headers", "*")
	context.Header("Access-Control-Max-Age", "172800")
	context.Header("Access-Control-Allow-Credentials", "false")
	context.Set("content-type", "application/json")

	if method == "OPTIONS" {
		context.Status(http.StatusOK)
		return
	}

	uid := uuid.NewString()
	// 请求打印
	data, err := httputil.DumpRequest(context.Request, false)
	if err != nil {
		logger.Error(err)
	} else {
		logger.Infof("\n------ START REQUEST %s ---------\n%s", uid, data)
	}

	//处理请求
	context.Next()

	// 结束处理
	logger.Infof("\n------ END REQUEST %s ---------", uid)
}

func panicError(ctx *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("response error: %v", r)
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": map[string]string{
					"message": fmt.Sprintf("%v", r),
				},
			})
		}
	}()

	//处理请求
	ctx.Next()
}
