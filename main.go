package helper

import (
	"context"
	"errors"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/sirupsen/logrus"
	"net/http/cookiejar"
	"runtime"
	"slices"
	"time"
	"you-helper/common"
	"you-helper/logger"
)

var (
	UserAgent string
)

func init() {
	common.InitCommon()
	logger.InitLogger("log", logrus.InfoLevel)
	switch runtime.GOOS {
	case "linux":
		UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.31 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
	case "darwin":
		UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
	case "windows":
		UserAgent = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
	default:
		UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
	}
}

func Clearance(ctx context.Context, proxies string) (cookies, lang string, err error) {
	ctx, cancel := InitChromium(ctx, proxies, UserAgent)
	defer cancel()

	// 进入主页
	retry := 3
	err = chromedp.Run(ctx,
		evaluateStealth(),
		taskLogger("进入主页..."),
		chromedp.Navigate("https://you.com"),
		//chromedp.Navigate("https://bot.sannysoft.com"),
		whileTimeout(30*time.Second, 5*time.Second, true, chromedp.ActionFunc(func(ctx context.Context) (err error) {
			if retry > 0 {
				retry--
				return chromedp.Run(ctx, chromedp.WaitVisible("#login-button"))
			}
			retry = 3
			return chromedp.Run(ctx, chromedp.ActionFunc(TryClickXY))
		})),
	)
	if err != nil {
		logger.Error(err)
		// 最后尝试一次
		timeout, cancelFunc := context.WithTimeout(ctx, 3*time.Second)
		_ = chromedp.Run(timeout, chromedp.ActionFunc(TryClickXY))
		cancelFunc()
		timeout, cancelFunc = context.WithTimeout(ctx, 3*time.Second)
		e := chromedp.Run(timeout, chromedp.WaitVisible("#login-button"))
		cancelFunc()
		if e == nil {
			err = nil
			goto label
		}

		_ = chromedp.Run(ctx, screenshot(nil))
		return
	}

label:
	keys := []string{
		"cf_clearance",
		"_cfuvid",
		"__cf_bm",
	}

	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) (err error) {
		cookieJar, err := network.GetCookies().Do(ctx)
		if err != nil {
			return
		}
		for _, cookie := range cookieJar {
			if slices.Contains(keys, cookie.Name) {
				cookies += cookie.Name + "=" + cookie.Value + "; "
			}
		}
		return
	}), chromedp.Evaluate(`navigator.languages.join(',') + ';q=0.9';`, &lang))
	if err != nil {
		logger.Error(err)
	}
	return
}

func Register(ctx context.Context, proxies string) (cookies string, err error) {
	ctx, cancel := InitChromium(ctx, proxies, UserAgent)
	defer cancel()

	// 进入主页
	retry := 3
	err = chromedp.Run(ctx,
		evaluateStealth(),
		taskLogger("进入主页..."),
		chromedp.Navigate("https://you.com"),
		//chromedp.Navigate("https://bot.sannysoft.com"),
		whileTimeout(30*time.Second, 5*time.Second, true, chromedp.ActionFunc(func(ctx context.Context) (err error) {
			if retry > 0 {
				retry--
				return chromedp.Run(ctx, chromedp.WaitVisible("#login-button"))
			}
			retry = 3
			return chromedp.Run(ctx, chromedp.ActionFunc(TryClickXY))
		})),
	)
	if err != nil {
		logger.Error(err)
		// 最后尝试一次
		timeout, cancelFunc := context.WithTimeout(ctx, 3*time.Second)
		_ = chromedp.Run(timeout, chromedp.ActionFunc(TryClickXY))
		cancelFunc()
		timeout, cancelFunc = context.WithTimeout(ctx, 3*time.Second)
		e := chromedp.Run(timeout, chromedp.WaitVisible("#login-button"))
		cancelFunc()
		if e == nil {
			err = nil
			goto label
		}

		_ = chromedp.Run(ctx, screenshot(nil))
		return
	}
label:
	// 获取邮箱
	jar, _ := cookiejar.New(nil)
	mail, err := randomMail(ctx, proxies, jar)
	if err != nil {
		return
	}

	// 点击登陆
	err = chromedp.Run(ctx,
		taskLogger("点击登陆["+mail+"]..."),
		chromedp.Click("#login-button"),
		whileTimeout(10*time.Second, 3*time.Second, true,
			chromedp.WaitVisible("#email-input"),
			chromedp.SendKeys("#email-input", mail)),
		chromedp.Click("#submit"),
	)

	if err != nil {
		return
	}

	// 等验证码
	logger.Info("等验证码...")
	messageCode, err := fetchMailMessage(ctx, proxies, mail, jar)
	if err != nil {
		return
	}
	logger.Infof("验证码为：%s", messageCode)

	err = chromedp.Run(ctx,
		taskLogger("输入验证码..."),
		whileTimeout(10*time.Second, 3*time.Second, true,
			chromedp.WaitVisible("div[direction=column] input[autocomplete=one-time-code]"),
			chromedp.SendKeys("div[direction=column] input[autocomplete=one-time-code]", messageCode)),
	)
	if err != nil {
		return
	}

	err = chromedp.Run(ctx,
		taskLogger("检查登陆状态..."),
		whileTimeout(18*time.Second, 3*time.Second, true,
			//chromedp.WaitVisible("#chat-layout-grid button[data-testid=explore-section]"),
			//chromedp.WaitNotVisible("#login-button"),
			notVisible("#login-button"),
		),
	)
	if err != nil {
		return
	}

	// 登陆成功，获取cookies
	keys := []string{
		"cf_clearance",
		"_cfuvid",
		"__cf_bm",
	}
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) (err error) {
		cookieJar, err := network.GetCookies().Do(ctx)
		if err != nil {
			return err
		}
		for _, cookie := range cookieJar {
			if !slices.Contains(keys, cookie.Name) {
				cookies += cookie.Name + "=" + cookie.Value + "; "
			}
		}
		return nil
	}))
	return
}

func TryClickXY(ctx context.Context) (err error) {
	var (
		iframe  *cdp.Node
		iframes []*cdp.Node

		rect map[string]interface{}
	)

	err = chromedp.Run(ctx, chromedp.Nodes("iframe", &iframes, chromedp.ByQuery))
	if err != nil {
		logger.Error(err)
		return
	}

	if len(iframes) == 0 {
		return errors.New("not iframe nodes")
	}

	iframe = iframes[0]
	logger.Info(iframe.Attribute("src"))
	err = chromedp.Run(ctx, chromedp.Evaluate(`{let {x,y} = document.querySelector('iframe').getBoundingClientRect(); let a={x,y}; a;}`, &rect))
	if err != nil {
		logger.Error(err)
		return
	}

	err = chromedp.Run(ctx, chromedp.MouseClickXY(rect["x"].(float64)+22+12, rect["y"].(float64)+23+12))
	if err != nil {
		logger.Error(err)
	}

	if err == nil {
		err = errors.New("trying to click XY")
	}
	return
}

//func main() {
//	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
//	defer cancel()
//
//	cookies, err := register(ctx)
//	if err != nil {
//		logger.Fatal(err)
//	}
//	logger.Info(cookies)
//
//	clearanceCookies, err := clearance(context.Background())
//	if err != nil {
//		logger.Fatal(err)
//	}
//
//	//response, err := emit.ClientBuilder().
//	//	Context(ctx).
//	//	Proxies("http://127.0.0.1:7890").
//	//	GET("https://you.com/api/user/getYouProState").
//	//	Ja3(ja3).
//	//	Header("user-agent", UserAgent).
//	//	Header("Cookie", emit.MergeCookies(cookies, clearanceCookies)).
//	//	Header("Connection", "keep-alive").
//	//	Header("Origin", "https://you.com").
//	//	Header("Accept-Language", "en-US,en;q=0.9").
//	//	Header("Referer", "https://you.com/?chatMode=default").
//	//	DoS(http.StatusOK)
//	//if err != nil {
//	//	logger.Fatal(err)
//	//}
//	//
//	//logger.Info(emit.TextResponse(response))
//}
