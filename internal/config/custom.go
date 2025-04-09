package config

type CustomConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func (s *CustomConfig) DefaultValues() error {
	return nil
}
