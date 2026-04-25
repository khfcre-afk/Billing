package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	App       AppConfig
	Security  SecurityConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	Ptero     PteroConfig
	Scheduler SchedulerConfig
}

type AppConfig struct {
	Env       string `envconfig:"APP_ENV" default:"development"`
	Port      string `envconfig:"APP_PORT" default:"8080"`
	PublicURL string `envconfig:"APP_PUBLIC_URL" default:"http://localhost:8080"`
}

type SecurityConfig struct {
	JWTSecret        string        `envconfig:"JWT_SECRET" required:"true"`
	JWTAccessTTL     time.Duration `envconfig:"JWT_ACCESS_TTL" default:"15m"`
	JWTRefreshTTL    time.Duration `envconfig:"JWT_REFRESH_TTL" default:"720h"`
	PaymentsDisabled bool          `envconfig:"PAYMENTS_DISABLED" default:"true"`
}

type PostgresConfig struct {
	Host     string `envconfig:"POSTGRES_HOST" default:"localhost"`
	Port     int    `envconfig:"POSTGRES_PORT" default:"5432"`
	User     string `envconfig:"POSTGRES_USER" required:"true"`
	Password string `envconfig:"POSTGRES_PASSWORD" required:"true"`
	DB       string `envconfig:"POSTGRES_DB" required:"true"`
	SSLMode  string `envconfig:"POSTGRES_SSLMODE" default:"disable"`
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DB, p.SSLMode)
}

func (p PostgresConfig) PgxConfig() (*pgx.ConnConfig, error) {
	return pgx.ParseConfig(p.DSN())
}

type RedisConfig struct {
	Addr     string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	Password string `envconfig:"REDIS_PASSWORD"`
}

type PteroConfig struct {
	BaseURL        string `envconfig:"PTERO_BASE_URL" required:"true"`
	AppAPIKey      string `envconfig:"PTERO_APP_API_KEY" required:"true"`
	ClientAPIKey   string `envconfig:"PTERO_CLIENT_API_KEY"`
	DefaultNodeID  int    `envconfig:"PTERO_DEFAULT_NODE_ID" default:"1"`
	DefaultNestID  int    `envconfig:"PTERO_DEFAULT_NEST_ID" default:"1"`
	DefaultEggID   int    `envconfig:"PTERO_DEFAULT_EGG_ID" default:"1"`
}

type SchedulerConfig struct {
	DailyChargeCron string `envconfig:"SCHEDULER_DAILY_CHARGE_CRON" default:"0 3 * * *"`
}

func Load() (Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return Config{}, fmt.Errorf("envconfig: %w", err)
	}
	if len(c.Security.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be at least 32 chars")
	}
	return c, nil
}
