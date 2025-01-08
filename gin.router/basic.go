package router

import (
	"chro-template/core"
	"chro-template/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"net/http/httputil"
	"strconv"
)

func Bind(port int, p string) {
	gin.SetMode(gin.ReleaseMode)
	route := gin.Default()
	{
		route.Use(gin.Recovery())
		route.Use(proxies(p))
		route.Use(cros)
	}
	route.GET("/demo", func(c *gin.Context) {
		if err := example.Examples(c); err != nil {
			logger.Fatal(err)
		}
	})
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
	}
	logger.Infof("\n------ START REQUEST %s ---------\n%s", uid, data)

	//处理请求
	context.Next()

	// 结束处理
	logger.Infof("\n------ END REQUEST %s ---------", uid)
}
