package helper

import (
	"chro-template/common"
	"chro-template/logger"
	"context"
	"github.com/chromedp/chromedp"
	"github.com/sirupsen/logrus"
)

var (
	proxies = ""
)

func Examples(ctx context.Context) (err error) {
	common.InitCommon()
	logger.InitLogger("log", logrus.InfoLevel)
	ctx, cancel := InitChromium(ctx, proxies, osUserAgent())
	defer cancel()

	// 进入主页
	err = chromedp.Run(ctx,
		evaluateStealth(),
		chromedp.Navigate("https://bot.sannysoft.com"),
		screenshot(nil),
		taskLogger("执行结束"),
	)
	return
}
