package example

import (
	"chro-template/common"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
)

func Examples(gtx *gin.Context) (err error) {
	ctx, cancel := common.InitChromium(gtx.Request.Context(), gtx.GetString("proxies"), common.UserAgent, "")
	defer cancel()

	// 进入主页
	err = chromedp.Run(ctx,
		common.EvaluateStealth(),
		chromedp.Navigate("https://bot.sannysoft.com"),
		common.Screenshot(nil),
		common.TaskLogger("执行结束"),
	)
	return
}
