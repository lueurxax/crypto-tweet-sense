package tweetfinder

import "github.com/kelseyhightower/envconfig"

type ConfigPool struct {
	Proxies []string `envconfig:"PROXIES"`
}

func GetConfigPool() ConfigPool {
	cfg := new(ConfigPool)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}

	return *cfg
}
