package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc"

	pb "github.com/mwasilew2/chatter/gen"
)

type chatClient struct {
	logger log.Logger
}

const (
	logLevelFlag = "loglevel"
)

func (s *chatClient) run(ctx *cli.Context) error {
	level.Info(s.logger).Log("msg", "running client")

	// adjust log level
	logLvl := ctx.Int(logLevelFlag)
	switch logLvl {
	case 0:
		s.logger = level.NewFilter(s.logger, level.AllowDebug())
	case 1:
		s.logger = level.NewFilter(s.logger, level.AllowInfo())
	case 2:
		s.logger = level.NewFilter(s.logger, level.AllowWarn())
	case 3:
		s.logger = level.NewFilter(s.logger, level.AllowError())
	}
	level.Debug(s.logger).Log("msg", "log level set", "level", logLvl)

	// set up grpc client
	port := ctx.Int("port")
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}
	defer conn.Close()
	c := pb.NewChatServerClient(conn)

	// print errors as they come in
	errChan := make(chan error)
	go func() {
		for e := range errChan {
			level.Error(s.logger).Log("msg", "error received", "err", e)
		}
	}()

	// read input and send messages
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		ctext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		r, err := c.Send(ctext, &pb.SendRequest{Message: line})
		if err != nil {
			errChan <- fmt.Errorf("failed to send message: %w", err)
			continue
		}
		level.Info(s.logger).Log("msg", "message successfully sent", "response", r)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	close(errChan)

	return nil
}
