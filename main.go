package main

import (
	"log"

	"github.com/mark3labs/mcp-go/server"
	"github.com/raychao-oao/ad-mcp/internal/config"
	ldapclient "github.com/raychao-oao/ad-mcp/internal/ldap"
	"github.com/raychao-oao/ad-mcp/internal/tools"
)

var Version = "0.1.0"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
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

	tools.Register(s, lc)

	log.Printf("ad-mcp %s listening on %s", Version, cfg.Addr)

	httpServer := server.NewStreamableHTTPServer(s)
	if err := httpServer.Start(cfg.Addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
