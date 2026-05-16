package mysqlconfig

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const DefaultParams = "charset=utf8mb4&parseTime=True&loc=Local"

const (
	defaultTimeout         = 3 * time.Second
	defaultReadTimeout     = 5 * time.Second
	defaultWriteTimeout    = 5 * time.Second
	defaultMaxOpenConns    = 50
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Params   string

	Timeout      time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func BuildDSN(cfg Config) (string, error) {
	host := strings.TrimSpace(cfg.Host)
	user := strings.TrimSpace(cfg.User)
	database := strings.TrimSpace(cfg.Database)
	if host == "" {
		return "", fmt.Errorf("db.host is required")
	}
	if user == "" {
		return "", fmt.Errorf("db.user is required")
	}
	if database == "" {
		return "", fmt.Errorf("db.database is required")
	}

	port := cfg.Port
	if port <= 0 {
		port = 3306
	}
	params, err := parseParams(defaultString(cfg.Params, DefaultParams))
	if err != nil {
		return "", err
	}

	return (&mysqldriver.Config{
		User:         user,
		Passwd:       cfg.Password,
		Net:          "tcp",
		Addr:         net.JoinHostPort(host, strconv.Itoa(port)),
		DBName:       database,
		Params:       params,
		Timeout:      defaultDuration(cfg.Timeout, defaultTimeout),
		ReadTimeout:  defaultDuration(cfg.ReadTimeout, defaultReadTimeout),
		WriteTimeout: defaultDuration(cfg.WriteTimeout, defaultWriteTimeout),
	}).FormatDSN(), nil
}

func ApplyPool(db *gorm.DB, cfg PoolConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = defaultMaxOpenConns
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = defaultMaxIdleConns
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(defaultDuration(cfg.ConnMaxLifetime, defaultConnMaxLifetime))
	sqlDB.SetConnMaxIdleTime(defaultDuration(cfg.ConnMaxIdleTime, defaultConnMaxIdleTime))
	return nil
}

func parseParams(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "?"))
	if raw == "" {
		return nil, nil
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, fmt.Errorf("db.params is invalid: %w", err)
	}
	params := make(map[string]string, len(values))
	for key, vals := range values {
		if len(vals) == 0 {
			params[key] = ""
			continue
		}
		params[key] = vals[len(vals)-1]
	}
	return params, nil
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
