package main

import (
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/urfave/cli/v2"
)

const (
	logLevelFlag = "loglevel"
)

func main() {
	var logger log.Logger

	c := &chatClient{}
	s := &chatServer{}
	b := &chatBoard{}

	app := &cli.App{
		Name:  "chatter",
		Usage: "",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  logLevelFlag,
				Value: 1,
				Usage: "Log level, 0-3, 0 debug, 1 info, 2 warn, 3 error",
			},
		},
		Before: func(ctx *cli.Context) error {

			// set up logger
			logLvl := ctx.Int(logLevelFlag)
			logger = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
			logger = level.NewFilter(logger, level.AllowAll())
			logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
			switch logLvl {
			case 0:
				logger = level.NewFilter(logger, level.AllowDebug())
			case 1:
				logger = level.NewFilter(logger, level.AllowInfo())
			case 2:
				logger = level.NewFilter(logger, level.AllowWarn())
			case 3:
				logger = level.NewFilter(logger, level.AllowError())
			}
			level.Debug(logger).Log("msg", "finished initializing logging")

			// inject logger into subcommands
			c.logger = logger
			s.logger = logger
			b.logger = logger

			return nil
		},
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
			{
				Name:   "board",
				Action: b.run,
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
