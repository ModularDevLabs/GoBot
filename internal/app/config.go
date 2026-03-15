package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

type ProcessConfig struct {
	DiscordToken        string
	DBPath              string
	BindAddr            string
	AdminPassword       string
	LogLevel            string
	DashboardRoleSecret map[string]string
}

const localConfigPath = ".modbot.config.json"

type persistedConfig struct {
	DiscordToken  string `json:"discord_token"`
	AdminPassword string `json:"admin_password"`
	DBPath        string `json:"db_path"`
	BindAddr      string `json:"bind_addr"`
	LogLevel      string `json:"log_level"`
}

func LoadProcessConfig() (ProcessConfig, error) {
	var cfg ProcessConfig

	flag.StringVar(&cfg.DiscordToken, "token", "", "Discord bot token")
	flag.StringVar(&cfg.DBPath, "db", "modbot.sqlite", "SQLite database path")
	flag.StringVar(&cfg.BindAddr, "bind", "127.0.0.1:8080", "HTTP bind address")
	flag.StringVar(&cfg.AdminPassword, "admin-pass", "", "Admin password for dashboard")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "Log level: info|debug")
	var roleSecretRaw string
	flag.StringVar(&roleSecretRaw, "dashboard-role-secrets", "", "JSON map of non-admin dashboard role credentials (example: {\"moderator\":\"mod-pass\",\"support\":\"support-pass\"})")
	flag.Parse()

	saved, err := loadPersistedConfig(localConfigPath)
	if err != nil {
		return cfg, fmt.Errorf("load local config: %w", err)
	}

	cfg.DiscordToken = firstNonEmpty(cfg.DiscordToken, os.Getenv("MODBOT_TOKEN"), saved.DiscordToken)
	cfg.DBPath = firstNonEmpty(cfg.DBPath, os.Getenv("MODBOT_DB"), saved.DBPath)
	cfg.BindAddr = firstNonEmpty(cfg.BindAddr, os.Getenv("MODBOT_BIND"), saved.BindAddr)
	cfg.AdminPassword = firstNonEmpty(cfg.AdminPassword, os.Getenv("MODBOT_ADMIN_PASS"), saved.AdminPassword)
	cfg.LogLevel = firstNonEmpty(cfg.LogLevel, os.Getenv("MODBOT_LOG_LEVEL"), saved.LogLevel)
	roleSecretRaw = firstNonEmpty(roleSecretRaw, os.Getenv("MODBOT_DASHBOARD_ROLE_SECRETS"))

	reader := bufio.NewReader(os.Stdin)
	prompted := false
	if cfg.DiscordToken == "" {
		cfg.DiscordToken = prompt(reader, "Discord bot token")
		prompted = true
	}
	if cfg.AdminPassword == "" {
		cfg.AdminPassword = promptSecret("Admin password")
		prompted = true
	}

	if cfg.DiscordToken == "" {
		return cfg, errors.New("missing Discord token (flag --token or MODBOT_TOKEN)")
	}
	if cfg.AdminPassword == "" {
		return cfg, errors.New("missing admin password (flag --admin-pass or MODBOT_ADMIN_PASS)")
	}
	cfg.DashboardRoleSecret = map[string]string{}
	if strings.TrimSpace(roleSecretRaw) != "" {
		if err := json.Unmarshal([]byte(roleSecretRaw), &cfg.DashboardRoleSecret); err != nil {
			return cfg, fmt.Errorf("parse dashboard role secrets JSON: %w", err)
		}
	}
	if prompted {
		if err := savePersistedConfig(localConfigPath, persistedConfig{
			DiscordToken:  cfg.DiscordToken,
			AdminPassword: cfg.AdminPassword,
			DBPath:        cfg.DBPath,
			BindAddr:      cfg.BindAddr,
			LogLevel:      cfg.LogLevel,
		}); err != nil {
			return cfg, fmt.Errorf("save local config: %w", err)
		}
	}

	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func promptSecret(label string) string {
	fmt.Printf("%s: ", label)
	secret, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(secret))
}

func loadPersistedConfig(path string) (persistedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return persistedConfig{}, nil
		}
		return persistedConfig{}, err
	}

	var cfg persistedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return persistedConfig{}, err
	}
	return cfg, nil
}

func savePersistedConfig(path string, cfg persistedConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
