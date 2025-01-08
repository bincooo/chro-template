package common

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chro-template/logger"
	"chro-template/plugins"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/spf13/viper"

	_ "embed"
	run "runtime"
)

var (
	UserAgent string
)

func init() {
	switch run.GOOS {
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

func switchPlugin(expr string) []byte {
	switch expr {
	case "nopecha":
		return plugins.Nopecha
	case "CaptchaSolver":
		return plugins.CaptchaSolver

		// more ...
	default:
		return nil
	}
}

func InitChromium(ctx context.Context, proxies, userAgent, userDir string, config *viper.Viper, plugins ...string) (context.Context, context.CancelFunc) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-css-animations", true),
		chromedp.Flag("disable-images", true),

		// UA
		chromedp.UserAgent(userAgent),

		// 窗口大小
		chromedp.WindowSize(800, 600),

		chromedp.NoFirstRun,

		// cert
		chromedp.IgnoreCertErrors,
	}

	// 用户目录
	if userDir != "" {
		opts = append(opts, chromedp.UserDataDir("tmp/"+userDir))
	}

	// 本地代理
	if proxies != "" {
		opts = append(opts, chromedp.ProxyServer(proxies))
	}

	headless := config.GetString("browser-less.headless")
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
	if config.GetBool("browser-less.disabled-gpu") {
		opts = append(opts, chromedp.DisableGPU)
	}

	// 代理ip白名单
	if list := config.GetStringSlice(""); len(list) > 0 {
		opts = append(opts, chromedp.Flag("browser-less.proxy-bypass-list", strings.Join(list, ",")))
	}

	// 插件装载
	if len(plugins) > 0 {
		opts = append(opts, InitExtensions(config, plugins...)...)
	}

	// 浏览器启动路径
	if p := config.GetString("browser-less.execPath"); p != "" {
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

func InitExtensions(config *viper.Viper, plugins ...string) []chromedp.ExecAllocatorOption {
	if len(plugins) == 0 {
		return nil
	}

	dir := config.GetString("browser-less.extension")
	if dir == "" {
		dir = "tmp/extension-plugins"
	}

	if run.GOOS == "windows" {
		matched, _ := regexp.MatchString("[a-zA-Z]:.+", dir)
		if !matched {
			pwd, _ := os.Getwd()
			dir = path.Join(pwd, dir)
		}
	} else {
		if dir[0] != '/' {
			pwd, _ := os.Getwd()
			dir = path.Join(pwd, dir)
		}
	}

	if !exists(dir) {
		_ = os.MkdirAll(dir, 0744)
	}

	var paths []string
	for _, plugin := range plugins {
		fp := filepath.Join(dir, plugin)
		pluginBytes := switchPlugin(plugin)

		if exists(fp) {
			paths = append(paths, fp)
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

		paths = append(paths, fp)
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
		fp := filepath.Join(folder, file.Name)

		// 如果是目录，就创建目录
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(fp, file.Mode()); err != nil {
				return err
			}
			continue
		}

		// 获取到 Reader
		fr, err := file.Open()
		if err != nil {
			return err
		}

		if strings.Contains(fp, "__MACOSX") {
			continue
		}

		if strings.Contains(fp, "manifest.fingerprint") {
			continue
		}

		// 创建要写出的文件对应的 Write
		fw, err := os.Create(fp)
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

func TaskLogger(message string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) (_ error) {
			logger.Info(message)
			return
		}),
	}
}

// 设定一个时间，轮询每个动作，直至超时或者执行成功
func WhileTimeout(timeout time.Duration, roundTimeout time.Duration, returnError bool, actions ...chromedp.Action) chromedp.ActionFunc {
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
func WithTimeout(timeout time.Duration, returnError bool, actions ...chromedp.Action) chromedp.ActionFunc {
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

func ClickXY(selector string) chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		return clickXY(ctx, selector)
	}
}

func clickXY(ctx context.Context, selector string) (err error) {
	var (
		rect map[string]interface{}
	)

	err = chromedp.Run(ctx, TaskLogger("click xy..."),
		chromedp.Evaluate(`{let {x,y} = document.querySelector("`+selector+`").getBoundingClientRect(); let a={x,y}; a;}`, &rect))
	if err != nil {
		logger.Error(err)
		return
	}

	err = chromedp.Run(ctx, chromedp.MouseClickXY(rect["x"].(float64)+22+12, rect["y"].(float64)+23+12))
	if err != nil {
		logger.Error(err)
	}
	return
}

func WaitCLickXY(selector string, timeout time.Duration, actions ...chromedp.Action) chromedp.ActionFunc {
	return WhileTimeout(timeout, 3*time.Second, true, append([]chromedp.Action{ClickXY(selector)}, actions...)...)
}

func Visible(selector string) chromedp.Tasks {
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

func NotVisible(selector string) chromedp.Tasks {
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

func NoReturnEvaluate(script string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, exp, err := runtime.Evaluate(script).Do(ctx)
			if err != nil {
				return err
			}
			if exp != nil {
				return exp
			}
			return nil
		}),
	}
}

func EvaluateStealth() chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		_, err = page.AddScriptToEvaluateOnNewDocument(plugins.StealthJs).Do(ctx)
		return
	}
}

func EvaluateHook() chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		_, err = page.AddScriptToEvaluateOnNewDocument(plugins.HookJs).Do(ctx)
		return
	}
}

func EvaluateHookJS(hookJS string) chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		_, err = page.AddScriptToEvaluateOnNewDocument(hookJS).Do(ctx)
		return
	}
}

func EvaluateCallbackManager(events ...string) chromedp.EvaluateAction {
	content := ""
	for _, event := range events {
		content += fmt.Sprintf("\nwindow.CallbackManager.register('%s', function(data) { this.data['%s'] = {ok: true, args: data}; });", event, event)
	}
	return chromedp.Evaluate(`
            // 创建一个回调管理器
            window.CallbackManager = {
                callbacks: {},
				data: {},
                // 注册回调
                register(id, callback) {
                    this.callbacks[id] = callback;
                },
                // 执行回调
                execute(id, data) {
                    if (this.callbacks[id]) {
                        return this.callbacks[id]?.call(this, data);
                    }
                    throw new Error('event "' + id + '" not found');
                },
				callback(id, timeout) {
					return new Promise((resolve, inject) => {
						if (timeout > 0) {
							let inj = false;
							setTimeout(() => { inj = true }, timeout);
							let timer = setInterval(() => {
								if (inj) {
									inject(new Error('timeout'));
									clearInterval(timer);
									return;
								}
								if (window.CallbackManager.data[id]) {
									const data = window.CallbackManager.data[id];
									clearInterval(timer);
									if (data.ok) {
										resolve(data.args);
									} else {
										inject(new Error('not data'));
									}
								}
							}, timeout);
							return;
						}

						if (window.CallbackManager.data[id]?.ok) {
							resolve(window.CallbackManager.data[id]?.args);
							return;
						}
						inject(new Error('not data'));
					});
				}
            };
            // 注册一些示例回调`+content, nil)
}

func EvaluateCallback[T any](name string, timeout time.Duration, result T) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		return chromedp.Evaluate(fmt.Sprintf("window.CallbackManager.callback('%s', %d)", name, timeout), &result, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			if timeout > 0 {
				p = p.WithTimeout(runtime.TimeDelta(timeout))
			}
			return p.WithAwaitPromise(true)
		}).Do(ctx)
	}
}

func Assert(condition func() bool, messages ...string) chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		if !condition() {
			err = errors.New("assert condition is false")
			message := strings.Join(messages, " ")
			if message != "" {
				err = errors.New(message)
			}
		}
		return
	}
}

func Screenshot(result chan string) chromedp.ActionFunc {
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
