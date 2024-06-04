package main

import (
	"chro-template/common"
	"chro-template/logger"
	"context"
	"github.com/chromedp/chromedp"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
)

func examples(ctx context.Context) (err error) {
	common.InitCommon()
	logger.InitLogger("log", logrus.InfoLevel)
	ctx, cancel := InitChromium(ctx, userAgent, true)
	defer cancel()

	// 进入主页
	err = chromedp.Run(ctx,
		chromedp.Navigate("https://you.com"),
		whileTimeout(120*time.Second, 5*time.Second, true, chromedp.WaitVisible("#login-button")),
		taskLogger("执行结束"),
	)
	if err != nil {
		_ = chromedp.Run(ctx, screenshot(nil))
		return
	}

	return
}

func main() {
	if err := examples(context.Background()); err != nil {
		logger.Fatal(err)
	}
}
