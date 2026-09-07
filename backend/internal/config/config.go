package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Env		string
	Port	int
	DataDir	string
	CORSOrigin	string
}

func Load() (Config, error) {
	c := Config{
		Env: 		env("RELIC_ENV", "dev"),
		DataDir: 	env("RELIC_DATA_DIR", "./data"),	
		CORSOrigin: env("RELIC_CORS_ORIGIN", "http://localhost:5137"),
	}

	p, err := strconv.Atoi(env("RELIC_PORT", "8000"))
	if err != nil {
		return c, fmt.Errorf("RELIC_PORT: %w", err)		
	}
	c.Port = p

	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return c, fmt.Errorf("data dir: %w", err)
	}

	return c, nil
}

func (c Config) Dev() bool { return c.Env == "dev" }

func env(k, def string) string {
	if v:= os.Getenv(k); v != "" { return v }
	return def
}