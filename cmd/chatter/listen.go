package main

import (
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/urfave/cli/v2"
)

type chatBoard struct {
	logger log.Logger
}

func (s *chatBoard) run(ctx *cli.Context) error {
	level.Debug(s.logger).Log("msg", "starting board")
	return nil
}
