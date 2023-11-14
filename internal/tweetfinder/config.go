package tweetfinder

import "github.com/kelseyhightower/envconfig"

type ConfigPool struct {
	XCreds          map[string]string `envconfig:"X_CREDS" required:"true"`
	XConfirmation   []string          `envconfig:"X_CONFIRMATION"`
	CookiesFilename string            `envconfig:"COOKIES_FILENAME" default:"cookies.json"`
}

func GetConfigPool() ConfigPool {
	cfg := new(ConfigPool)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}
	return *cfg
}
