package logic

type Logic struct {
	cfg *Config
}

func New(cfg *Config) *Logic {
	return &Logic{cfg: cfg}
}
