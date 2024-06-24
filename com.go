package main

import (
	"archive/zip"
	"bytes"
	"chro-template/config"
	"chro-template/logger"
	"context"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func InitChromium(ctx context.Context, proxies, userAgent string) (context.Context, context.CancelFunc) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),

		// UA
		chromedp.UserAgent(userAgent),
		// 浏览器启动路径
		//chromedp.ExecPath("/usr/bin/microsoft-edge"),

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

	headless := config.Config.GetString("serverless.headless")
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
	if config.Config.GetBool("serverless.disabled-gpu") {
		opts = append(opts, chromedp.DisableGPU)
	}

	// 代理ip白名单
	if list := config.Config.GetStringSlice(""); len(list) > 0 {
		opts = append(opts, chromedp.Flag("serverless.proxy-bypass-list", strings.Join(list, ",")))
	}

	// 插件装载
	opts = append(opts, InitExtensions("nopecha")...)

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

	dir := config.Config.GetString("serverless.extension")
	if dir == "" {
		dir = "/var/tmp/extension-plugins"
	}

	if !exists(dir) {
		_ = os.MkdirAll(dir, 0744)
	}

	var paths []string
	for _, plugin := range plugins {
		path := filepath.Join(dir, plugin)
		if !exists("plugins/" + plugin + ".1") {
			continue
		}

		f, err := os.Open("plugins/" + plugin + ".1")
		if err != nil {
			logger.Error(err)
			continue
		}

		if err = fix(f); err != nil {
			logger.Error(err)
			_ = f.Close()
			continue
		}

		unzip, err := newZipReader(f)
		if err != nil {
			logger.Error(err)
			_ = f.Close()
			continue
		}

		if err = unzipToDir(unzip, dir); err != nil {
			logger.Error(err)
			_ = f.Close()
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

func newZipReader(f *os.File) (*zip.Reader, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return zip.NewReader(f, fi.Size())
}

func fix(f *os.File) error {
	magic := make([]byte, 8)
	n, err := f.Read(magic)
	if err != nil {
		return err
	}

	if n > 8 && bytes.Equal(magic, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) {
		n, err = f.Write([]byte{0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x00, 0x00})
		if err != nil {
			return err
		}
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

func evaluateStealth() chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		_, err = page.AddScriptToEvaluateOnNewDocument(stealthJs).Do(ctx)
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
