package helper

import (
	_ "embed"
	"github.com/chromedp/cdproto/cdp"

	"archive/zip"
	"bytes"
	"chro-template/config"
	"chro-template/logger"
	"context"
	"errors"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	rt "runtime"
)

var (

	//go:embed plugins/nopecha.1
	nopecha []byte
)

func switchPlugin(expr string) []byte {
	switch expr {
	case "nopecha":
		return nopecha
		// more ...
	default:
		return nil
	}
}

func InitChromium(ctx context.Context, proxies, userAgent string) (context.Context, context.CancelFunc) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),

		// UA
		chromedp.UserAgent(userAgent),

		// 用户目录
		//chromedp.UserDataDir("./user-dir"),

		// 窗口大小
		//chromedp.WindowSize(800, 600),

		chromedp.NoFirstRun,
	}

	// 本地代理
	if proxies != "" {
		opts = append(opts, chromedp.ProxyServer(proxies))
	}

	headless := config.Config.GetString("browser-less.headless")
	if headless != "" {
		// 设置为false，就是不使用无头模式
		switch headless {
		case "new":
			opts = append(opts, chromedp.Flag("headless", headless))
		case "true":
			opts = append(opts, chromedp.Flag("headless", true))
		case "false":
			opts = append(opts, chromedp.Flag("headless", false))
		}
	}

	// 关闭GPU加速
	if config.Config.GetBool("browser-less.disabled-gpu") {
		opts = append(opts, chromedp.DisableGPU)
	}

	// 代理ip白名单
	if list := config.Config.GetStringSlice(""); len(list) > 0 {
		opts = append(opts, chromedp.Flag("browser-less.proxy-bypass-list", strings.Join(list, ",")))
	}

	// 插件装载
	opts = append(opts, InitExtensions("nopecha")...)

	// 浏览器启动路径
	if p := config.Config.GetString("serverless.execPath"); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
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

func InitExtensions(plugins ...string) []chromedp.ExecAllocatorOption {
	if len(plugins) == 0 {
		return nil
	}

	dir := config.Config.GetString("browser-less.extension")
	if dir == "" {
		dir = "/var/tmp/extension-plugins"
	}

	if !exists(dir) {
		_ = os.MkdirAll(dir, 0744)
	}

	var paths []string
	for _, plugin := range plugins {
		path := filepath.Join(dir, plugin)
		pluginBytes := switchPlugin(plugin)

		if exists(path) {
			paths = append(paths, path)
			continue
		}

		if err := fix(pluginBytes); err != nil {
			logger.Error(err)
			continue
		}

		unzip, err := newZipReader(pluginBytes)
		if err != nil {
			logger.Error(err)
			continue
		}

		if err = unzipToDir(unzip, dir); err != nil {
			logger.Error(err)
			continue
		}

		paths = append(paths, path)
	}

	return []chromedp.ExecAllocatorOption{
		chromedp.Flag("disable-extensions-except", strings.Join(paths, ",")),
		chromedp.Flag("load-extension", strings.Join(paths, ",")),
		chromedp.Flag("disable-extensions", false),
	}
}

func unzipToDir(zr *zip.Reader, folder string) error {
	// 遍历 zr ，将文件写入到磁盘
	for _, file := range zr.File {
		path := filepath.Join(folder, file.Name)

		// 如果是目录，就创建目录
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return err
			}
			continue
		}

		// 获取到 Reader
		fr, err := file.Open()
		if err != nil {
			return err
		}

		if strings.Contains(path, "/__MACOSX/") {
			continue
		}

		if strings.Contains(path, "manifest.fingerprint") {
			continue
		}

		// 创建要写出的文件对应的 Write
		fw, err := os.Create(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(fw, fr)
		if err != nil {
			return err
		}

		_ = fw.Close()
		_ = fr.Close()
	}

	return nil
}

func newZipReader(pluginBytes []byte) (*zip.Reader, error) {
	return zip.NewReader(bytes.NewReader(pluginBytes), int64(len(pluginBytes)))
}

func fix(pluginBytes []byte) error {
	if len(pluginBytes) <= 8 {
		return errors.New("plugin bytes too short")
	}
	if bytes.Equal(pluginBytes[:8], []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) {
		pluginBytes[0] = 0x50
		pluginBytes[0] = 0x4B
		pluginBytes[0] = 0x03
		pluginBytes[0] = 0x04
		pluginBytes[0] = 0x14
		pluginBytes[0] = 0x00
		pluginBytes[0] = 0x00
		pluginBytes[0] = 0x00
	}
	return nil
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
				time.Sleep(time.Second)
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

func visible(selector string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			obj, exp, err := runtime.Evaluate("document.querySelector('" + selector + "')").Do(ctx)
			if err != nil {
				return err
			}
			if exp != nil {
				return exp
			}

			if obj.ObjectID == "" {
				return errors.New("not visible")
			}
			return nil
		}),
	}
}

func notVisible(selector string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			obj, exp, err := runtime.Evaluate("document.querySelector('" + selector + "')").Do(ctx)
			if err != nil {
				return err
			}
			if exp != nil {
				return exp
			}

			if obj.ObjectID != "" {
				return errors.New("visible")
			}
			return nil
		}),
	}
}

func clickXY(ctx context.Context, selector string, offset struct{ x, y float64 }) (err error) {
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

	if selector == "" {
		selector = "body"
	}
	err = chromedp.Run(ctx, chromedp.Evaluate("{let {x,y} = document.querySelector(`"+selector+"`).getBoundingClientRect(); let a={x,y}; a;}", &rect))
	if err != nil {
		logger.Error(err)
		return
	}

	err = chromedp.Run(ctx, chromedp.MouseClickXY(rect["x"].(float64)+offset.x, rect["y"].(float64)+offset.y))
	if err != nil {
		logger.Error(err)
	}

	if err == nil {
		err = errors.New("trying to click XY")
	}
	return
}

func evaluateStealth() chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		_, err = page.AddScriptToEvaluateOnNewDocument(stealthJs).Do(ctx)
		return
	}
}

func osUserAgent() string {
	switch rt.GOOS {
	case "linux":
		return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.31 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
	case "darwin":
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
	case "windows":
		return "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
	default:
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0"
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
	return err == nil || os.IsExist(err)
}
