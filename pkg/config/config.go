package config

type JiraToolConfig struct {
	Teams []TeamConfig `yaml:"teams"`
}

type TeamConfig struct {
	Name       string   `yaml:"name"`
	Components []string `yaml:"components"`
}
