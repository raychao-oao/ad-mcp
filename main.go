package main

import (
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/raychao-oao/ad-mcp/internal/config"
	ldapclient "github.com/raychao-oao/ad-mcp/internal/ldap"
	"github.com/raychao-oao/ad-mcp/internal/tools"
	"github.com/raychao-oao/mcp-policy/pkg/yamlengine"
)

var Version = "0.1.2"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var engine *yamlengine.Engine
	if cfg.PolicyFile != "" {
		if _, statErr := os.Stat(cfg.PolicyFile); statErr == nil {
			engine, err = yamlengine.LoadFile(cfg.PolicyFile)
			if err != nil {
				log.Fatalf("policy: %v", err)
			}
			log.Printf("policy loaded from %s", cfg.PolicyFile)
		} else {
			log.Printf("policy file not found (%s), running without policy enforcement", cfg.PolicyFile)
		}
	}

	lc, err := ldapclient.New(cfg.LDAP)
	if err != nil {
		log.Fatalf("ldap: %v", err)
	}
	defer lc.Close()

	s := server.NewMCPServer(
		"ad-mcp",
		Version,
		server.WithToolCapabilities(false),
	)

	tools.Register(s, lc, engine)

	log.Printf("ad-mcp %s listening on %s", Version, cfg.Addr)

	httpServer := server.NewStreamableHTTPServer(s)
	if err := httpServer.Start(cfg.Addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
