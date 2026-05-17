package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Server struct {
	Port int `yaml:"port"`
}

type Database struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	SSLMode  string `yaml:"sslmode"`
}

type Auth struct {
	JWTSecret       string        `yaml:"jwt_secret"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
}

type Bootstrap struct {
	OwnerEmail    string `yaml:"owner_email"`
	OwnerPassword string `yaml:"owner_password"`
}

type Printer struct {
	Enabled bool          `yaml:"enabled"`
	Address string        `yaml:"address"`        // host:port (raw TCP, typically port 9100)
	Width   int           `yaml:"width"`          // chars per line (32 for 58mm, 48 for 80mm)
	Timeout time.Duration `yaml:"timeout"`        // dial+write timeout
	Header  []string      `yaml:"header"`         // shop name/address lines printed on top
	Footer  []string      `yaml:"footer"`         // closing lines (e.g. "Thank you!")
	OpenDrawer bool       `yaml:"open_drawer"`    // send drawer-kick command after print
}

type Config struct {
	Server    Server    `yaml:"server"`
	Database  Database  `yaml:"database"`
	Auth      Auth      `yaml:"auth"`
	Bootstrap Bootstrap `yaml:"bootstrap"`
	Printer   Printer   `yaml:"printer"`
}

func (d Database) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("APOTECH_CONFIG")
		if path == "" {
			path = "config.yaml"
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	var c Config
	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}
	return &c, nil
}

func MustLoad() *Config {
	c, err := Load("")
	if err != nil {
		panic(err)
	}
	return c
}
