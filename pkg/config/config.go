package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	SubPub SubPubConfig `mapstructure:"subpub"`
	Log    LogConfig    `mapstructure:"log"`
}

type ServerConfig struct {
	Host                 string `mapstructure:"host"`
	Port                 int    `mapstructure:"port"`
	MaxConcurrentStreams int32  `mapstructure:"max_concurrent_streams"`
}

type SubPubConfig struct {
	BufferSize         int    `mapstructure:"buffer_size"`
	DropStrategy       string `mapstructure:"drop_strategy"`
	WorkerPoolSize     int    `mapstructure:"worker_pool_size"`
	SubscriptionBuffer int    `mapstructure:"subscription_buffer"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func setDefaults() {
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 50051)
	viper.SetDefault("server.max_concurrent_streams", 100)

	viper.SetDefault("subpub.buffer_size", 100)
	viper.SetDefault("subpub.drop_strategy", "newest")
	viper.SetDefault("subpub.worker_pool_size", 10)
	viper.SetDefault("subpub.subscription_buffer", 100)

	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
}
