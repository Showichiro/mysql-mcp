package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Showichiro/mysql-mcp/internal/config"
	"github.com/Showichiro/mysql-mcp/internal/mysqlmcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	app, err := mysqlmcp.New(cfg)
	if err != nil {
		log.Fatalf("server init error: %v", err)
	}
	defer app.Close()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mysql-mcp",
		Version: version,
	}, nil)
	app.Register(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.New(os.Stderr, "", log.LstdFlags).Fatal(err)
	}
}
