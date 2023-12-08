package watcher

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Queries        []string      `envconfig:"QUERIES" default:"bitcoin,ethereum,ripple,crypto,cryptocurrenc,altcoin,BTC"`
	CleanInterval  time.Duration `envconfig:"CLEAN_INTERVAL" default:"10m"`
	TooOld         time.Duration `envconfig:"TOO_OLD" default:"9h"`
	SearchInterval time.Duration `envconfig:"SEARCH_INTERVAL" default:"1m"`
}

func GetConfig() *Config {
	cfg := new(Config)
	if err := envconfig.Process("WATCHER", cfg); err != nil {
		panic(err)
	}

	return cfg
}
