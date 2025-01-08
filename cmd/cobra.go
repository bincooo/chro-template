package main

import (
	"chro-template/common"
	"chro-template/gin.router"
	"chro-template/logger"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	port     int
	logLevel = "info"
	logPath  = "log"
	version  = "v0.0.1"
	proxies  = ""

	cmd = &cobra.Command{
		Short:   "chrome 自动化模版",
		Long:    "项目地址：https://github.com/bincooo/chro-template",
		Use:     "chro-template",
		Version: version,
		Run: func(cmd *cobra.Command, args []string) {
			common.InitCommon()
			logger.InitLogger(logPath, LogLevel())
			router.Bind(port, proxies)
		},
	}
)

func main() {
	cmd.PersistentFlags().IntVar(&port, "port", 8080, "服务端口 port")
	cmd.PersistentFlags().StringVar(&logLevel, "log", logLevel, "日志级别: trace|debug|info|warn|error")
	cmd.PersistentFlags().StringVar(&logPath, "log-path", logPath, "日志路径")
	cmd.PersistentFlags().StringVar(&proxies, "proxies", proxies, "本地代理地址")
	_ = cmd.Execute()
}

func LogLevel() logrus.Level {
	switch logLevel {
	case "trace":
		return logrus.TraceLevel
	case "debug":
		return logrus.DebugLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	default:
		return logrus.InfoLevel
	}
}
