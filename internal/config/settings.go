package config

import "os"

type App struct {
	Port string
}

type Config struct {
	App
}

func NewConfigFactory() *Config {
	return &Config{
		App{
			Port: os.Getenv("ABBIE_PORT"),
		},
	}
}
