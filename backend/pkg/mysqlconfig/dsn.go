package mysqlconfig

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"
)

const DefaultParams = "charset=utf8mb4&parseTime=True&loc=Local"

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Params   string
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
		User:   user,
		Passwd: cfg.Password,
		Net:    "tcp",
		Addr:   net.JoinHostPort(host, strconv.Itoa(port)),
		DBName: database,
		Params: params,
	}).FormatDSN(), nil
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
