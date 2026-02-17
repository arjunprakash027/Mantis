package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Pipelines struct {
		Discovery struct {
			Enabled         bool `yaml:"enabled"`
			IntervalMinutes int  `yaml:"interval_minutes"`
		} `yaml:"discovery"`
		Orderbook struct {
			Enabled bool     `yaml:"enabled"`
			Markets []string `yaml:"markets"`
		} `yaml:"orderbook"`
	} `yaml:"pipelines"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
