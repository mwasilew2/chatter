package main

import (
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/urfave/cli/v2"
)

func main() {
	var logger log.Logger
	logger = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
	logger = level.NewFilter(logger, level.AllowAll())
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	level.Debug(logger).Log("msg", "Finished initializing logging.")

	c := &chatClient{
		logger: logger,
	}
	s := &chatServer{
		logger: logger,
	}

	app := &cli.App{
		Name:  "chatter",
		Usage: "",
		Commands: []*cli.Command{
			{
				Name:   "client",
				Action: c.run,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "port",
						Value: 8080,
						Usage: "Port to connect to",
					},
					&cli.IntFlag{
						Name:  logLevelFlag,
						Value: 0,
						Usage: "Log level, 0-3, 0 debug, 1 info, 2 warn, 3 error",
					},
				},
			},
			{
				Name:   "server",
				Action: s.run,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "port",
						Value: 8080,
						Usage: "Port to connect to",
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		level.Error(logger).Log("err", fmt.Errorf("error running the command: %w", err))
		os.Exit(-1)
	}
}
