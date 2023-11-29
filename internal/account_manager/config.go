package account_manager

import "github.com/kelseyhightower/envconfig"

type Config struct {
	Login           string `envconfig:"LOGIN" required:"true"`
	Password        string `envconfig:"PASSWORD" required:"true"`
	Confirmation    string `envconfig:"CONFIRMATION"`
	CookiesFilename string `envconfig:"COOKIES_FILENAME"`
}

func GetConfig() Config {
	cfg := new(Config)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}

	return *cfg
}
