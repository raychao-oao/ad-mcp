package config

import (
	"fmt"
	"os"
)

type Config struct {
	Addr       string
	PolicyFile string
	LDAP       LDAPConfig
}

type LDAPConfig struct {
	URL      string // ldap://dc.example.com:389 or ldaps://...
	BindDN   string // service account DN
	Password string // service account password
	BaseDN   string // search base DN
}

func Load() (*Config, error) {
	c := &Config{
		Addr:       envOr("AD_MCP_ADDR", ":8080"),
		PolicyFile: envOr("AD_MCP_POLICY_FILE", "/etc/ad-mcp/policy.yaml"),
		LDAP: LDAPConfig{
			URL:      os.Getenv("AD_MCP_LDAP_URL"),
			BindDN:   os.Getenv("AD_MCP_LDAP_BIND_DN"),
			Password: os.Getenv("AD_MCP_LDAP_BIND_PASSWORD"),
			BaseDN:   os.Getenv("AD_MCP_LDAP_BASE_DN"),
		},
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.LDAP.URL == "" {
		return fmt.Errorf("AD_MCP_LDAP_URL is required")
	}
	if c.LDAP.BindDN == "" {
		return fmt.Errorf("AD_MCP_LDAP_BIND_DN is required")
	}
	if c.LDAP.Password == "" {
		return fmt.Errorf("AD_MCP_LDAP_BIND_PASSWORD is required")
	}
	if c.LDAP.BaseDN == "" {
		return fmt.Errorf("AD_MCP_LDAP_BASE_DN is required")
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
