package config

import (
	"bytes"
	"chro-template/common"
	"github.com/spf13/viper"
	"os"
)

var (
	Config *viper.Viper
)

func LoadConfig() (*viper.Viper, error) {
	dateBytes, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}

	vip := viper.New()
	vip.SetConfigType("yaml")
	if err = vip.ReadConfig(bytes.NewReader(dateBytes)); err != nil {
		return nil, err
	}

	return vip, nil
}

func init() {
	common.AddInitialized(func() {
		config, err := LoadConfig()
		if err != nil {
			panic(err)
		}
		Config = config
	})
}
