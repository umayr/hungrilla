package conf

import (
	"os"

	"github.com/spf13/viper"

	log "github.com/Sirupsen/logrus"
	prefixed "github.com/umayr/logrus-prefixed-formatter"
)

const (
	Development = "development"
	Staging     = "staging"
	Production  = "production"
)

type Conf struct {
	BaseURL  string `yaml:"base-url"`
	City     string `yaml:"city"`
	MaxPages int    `yaml:"max-pages"`
}

var conf Conf

func init() {
	env := os.Getenv("ENV")
	if env == "" {
		env = Development
		os.Setenv("ENV", env)
	}

	switch env {
	case Development:
		log.SetLevel(log.DebugLevel)
		log.SetFormatter(&prefixed.TextFormatter{
			Colors: &prefixed.Colors{
				Prefix: "59+b",
				Debug:  "253",
				Warn:   "178",
				Info:   "74",
				Error:  "9",
			},
			ShortTimestamp: true,
		})
	case Staging:
		log.SetLevel(log.InfoLevel)
	case Production:
		log.SetLevel(log.ErrorLevel)
		log.SetFormatter(&log.JSONFormatter{})
	}

	viper.SetConfigName(env)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("conf/")

	err := viper.ReadInConfig()

	if err != nil {
		panic(err)
	}

	err = viper.Unmarshal(&conf)
	if err != nil {
		panic(err)
	}
}

// Get the configuration based on the `ENV` environment variable
// If ENV hasn't been set, then it gets the `development` configs
func Get() *Conf {
	return &conf
}
