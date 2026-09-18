package config

type Config struct {
	Profile string `json:"profile"`
	Region  string `json:"region"`
}

func Default() Config {
	return Config{
		Profile: "infraresc",
		Region:  "",
	}
}
