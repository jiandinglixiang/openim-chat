package config

import (
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/openimsdk/tools/errs"
	"github.com/spf13/viper"
)

func LoadConfig(path string, envPrefix string, config any) error {
	v := viper.New()
	fmt.Println("读取文件：", path)
	v.SetConfigFile(path)
	v.SetEnvPrefix(envPrefix)

	// if err := v.ReadInConfig(); err != nil {
	// 	return errs.WrapMsg(err, "failed to read config file", "path", path, "envPrefix", envPrefix)
	// }

	// // 打印从配置文件中读取到的变量
	// fmt.Println("Configuration settings before AutomaticEnv:")
	// for key, value := range v.AllSettings() {
	// 	fmt.Printf("配置输出~%s: %v\n", key, value)
	// }

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return errs.WrapMsg(err, "failed to read config file", "path", path, "envPrefix", envPrefix)
	}

	if err := v.Unmarshal(config, func(config *mapstructure.DecoderConfig) {
		config.TagName = "mapstructure"
	}); err != nil {
		return errs.WrapMsg(err, "failed to unmarshal config", "path", path, "envPrefix", envPrefix)
	}

	return nil
}
