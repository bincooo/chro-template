package main

import (
	"chro-template/logger"
	"context"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"time"
)

func InitChromium(ctx context.Context, userAgent string) (context.Context, context.CancelFunc) {
	path := filepath.Join("plugins/nopecha")
	opts := []chromedp.ExecAllocatorOption{
		//chromedp.DisableGPU,
		chromedp.Flag("headless", false), // 设置为false，就是不使用无头模式
		// 本地代理
		chromedp.ProxyServer("http://127.0.0.1:7890"),
		//chromedp.Flag("proxy-bypass-list", "<-loopback>"),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		// 插件装载
		chromedp.Flag("disable-extensions-except", path),
		chromedp.Flag("disable-extensions", false),
		chromedp.Flag("load-extension", path),
		// UA
		chromedp.UserAgent(userAgent),
		// 浏览器启动路径
		//chromedp.ExecPath("/usr/bin/microsoft-edge"),
	}

	opts = append(chromedp.DefaultExecAllocatorOptions[:], opts...)
	chromiumCtx, _ := chromedp.NewExecAllocator(ctx, opts...)

	ctx, cancel := chromedp.NewContext(
		chromiumCtx,
		chromedp.WithLogf(logger.Infof),
		chromedp.WithDebugf(logger.Debugf),
		chromedp.WithErrorf(logger.Errorf),
	)

	return ctx, cancel
}

func taskLogger(message string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) (_ error) {
			logger.Info(message)
			return
		}),
	}
}

// 设定一个时间，轮询每个动作，直至超时或者执行成功
func whileTimeout(timeout time.Duration, roundTimeout time.Duration, returnError bool, actions ...chromedp.Action) chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		timer := time.After(timeout)
		for {
			select {
			case <-timer:
				if !returnError {
					return nil
				}
				if err != nil {
					return
				}
				return context.DeadlineExceeded
			default:
				t, cancel := context.WithTimeout(ctx, roundTimeout)
				if err = chromedp.Run(t, actions...); err == nil {
					cancel()
					return nil
				}
				cancel()
			}
		}
	}
}

// 设定一个时间，直至超时或者执行成功
func withTimeout(timeout time.Duration, returnError bool, actions ...chromedp.Action) chromedp.ActionFunc {
	// 执行动作
	return func(ctx context.Context) (err error) {
		t, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err = chromedp.Run(t, actions...); err == nil {
			return nil
		}
		if !returnError {
			return nil
		}
		return
	}
}

func screenshot(result chan string) chromedp.ActionFunc {
	// 执行动作
	var screenshotBytes []byte
	return func(ctx context.Context) (err error) {
		err = chromedp.Run(ctx, chromedp.CaptureScreenshot(&screenshotBytes))
		if err == nil {
			if !exists("tmp") {
				_ = os.Mkdir("tmp", 0744)
			}

			file := "tmp/screenshot-" + uuid.NewString() + ".png"
			e := os.WriteFile(file, screenshotBytes, 0744)
			if e != nil {
				logger.Error("screenshot failed: ", e)
				return
			}

			logger.Info("screenshot file: ", file)
			if result != nil {
				result <- file
			}
		}
		return
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return os.IsExist(err)
}
