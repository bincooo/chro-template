package main

import (
	"chro-template/common"
	"chro-template/logger"
	"context"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/sirupsen/logrus"
	"slices"
	"time"
)

const (
	proxies   = ""
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
)

func examples(ctx context.Context) (err error) {
	common.InitCommon()
	logger.InitLogger("log", logrus.InfoLevel)
	ctx, cancel := InitChromium(ctx, proxies, userAgent)
	defer cancel()

	// 进入主页
	err = chromedp.Run(ctx,
		evaluateStealth(),
		chromedp.Navigate("https://you.com"),
		//chromedp.Navigate("https://bot.sannysoft.com"),
		whileTimeout(100*time.Second, 3*time.Second, true, chromedp.WaitVisible("#login-button")),
		taskLogger("执行结束"),
	)
	if err != nil {
		_ = chromedp.Run(ctx, screenshot(nil))
		logger.Error(err)
		return
	}

	keys := []string{
		"cf_clearance",
		"_cfuvid",
		"__cf_bm",
	}
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) (err error) {
		cookies, err := network.GetCookies().Do(ctx)
		if err != nil {
			return err
		}
		var str string
		for _, cookie := range cookies {
			if slices.Contains(keys, cookie.Name) {
				str += cookie.Name + "=" + cookie.Value + "; "
			}
		}
		logger.Info(str)
		return nil
	}))
	if err != nil {
		logger.Error(err)
	}
	return
}

func main() {
	if err := examples(context.Background()); err != nil {
		logger.Fatal(err)
	}
}
