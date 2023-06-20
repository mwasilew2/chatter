package main

import (
	"os"

	"golang.org/x/exp/slog"

	"github.com/alecthomas/kong"
)

var programLevel = new(slog.LevelVar)

type cmdContext struct {
	Logger *slog.Logger
}

var kongApp struct {
	LogLevel int `short:"l" help:"Log level: 0 (debug), 1 (info), 2 (warn), 3 (error)" default:"1"`

	ChatServer ChatServerCmd `cmd:"" help:"Start a chat server."`
	Client     ChatClientCmd `cmd:"" help:"Start a chat client."`
	Board      ChatBoardCmd  `cmd:"" help:"Start a chat board."`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel}))
	slog.SetDefault(logger)

	kongCtx := kong.Parse(&kongApp,
		kong.Description("A simple chat application."),
		kong.UsageOnError(),
		kong.Vars{

			"version": "0.0.1", // TODO: Use goreleaser to set this?
		},
	)
	switch kongApp.LogLevel {
	case 0:
		programLevel.Set(slog.LevelDebug)
	case 1:
		programLevel.Set(slog.LevelInfo)
	case 2:
		programLevel.Set(slog.LevelWarn)
	case 3:
		programLevel.Set(slog.LevelError)
	}
	err := kongCtx.Run(&cmdContext{Logger: logger})
	kongCtx.FatalIfErrorf(err)
}
