package config

import "os"

type ServerConfig struct {
	APIPort             string
	DatabaseURL         string
	TestSuitesPath      string
	GiteaURL            string
	GiteaPublicURL      string
	GiteaOrg            string
	GiteaAdminUser      string
	GiteaAdminPassword  string
}

func LoadServerConfig() *ServerConfig {
	return &ServerConfig{
		APIPort:            getEnv("API_PORT", "8000"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		TestSuitesPath:     getEnv("TESTSUITES_PATH", "/var/ft_hackthon/testsuites"),
		GiteaURL:           getEnv("GITEA_URL", "http://gitea:3000"),
		GiteaPublicURL:     getEnv("GITEA_PUBLIC_URL", "http://localhost:3000"),
		GiteaOrg:           getEnv("GITEA_ORG", "moulinerie"),
		GiteaAdminUser:     getEnv("GITEA_ADMIN_USER", "ft_hackthon"),
		GiteaAdminPassword: getEnv("GITEA_ADMIN_PASSWORD", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
