package tweetfinder

import (
	"fmt"
	"net"

	"github.com/kelseyhightower/envconfig"
)

type ConfigProxies struct {
	Port      string   `envconfig:"PROXY_PORT"`
	Addresses []string `envconfig:"PROXY_ADDRESSES"`
	Username  string   `envconfig:"PROXY_USERNAME"`
	Password  string   `envconfig:"PROXY_PASSWORD"`
}

func GetConfigPool() ConfigProxies {
	cfg := new(ConfigProxies)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}

	return *cfg
}

func (c ConfigProxies) GetProxies() []string {
	data := make([]string, 0, len(c.Addresses))

	for _, addr := range c.Addresses {
		str := fmt.Sprintf("http://%s:%s@%s", c.Username, c.Password, net.JoinHostPort(addr, c.Port))
		data = append(data, str)
	}

	return data
}
