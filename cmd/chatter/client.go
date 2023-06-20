package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"golang.org/x/exp/slog"

	"github.com/muesli/cancelreader"
	pb "github.com/mwasilew2/chatter/gen"
	"github.com/oklog/run"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ChatClientCmd struct {
	// cli options
	Addr string `help:"address to connect to" default:":8080"`

	// Dependencies
	logger *slog.Logger
}

func (c *ChatClientCmd) Run(cmdCtx *cmdContext) error {
	c.logger = cmdCtx.Logger.With("component", "ChatClientCmd")
	c.logger.Info("starting chat client", "addr", c.Addr)

	// set up grpc client
	conn, err := grpc.Dial(c.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}
	defer conn.Close()
	pbClient := pb.NewChatServerClient(conn)

	// run goroutines
	g := run.Group{}
	errChan := make(chan error)

	// listen for termination signals
	osSigChan := make(chan os.Signal)
	signal.Notify(osSigChan, os.Kill, os.Interrupt)
	done := make(chan struct{})
	g.Add(func() error {
		select {
		case sig := <-osSigChan:
			c.logger.Debug("caught signal", "signal", sig.String())
			return fmt.Errorf("received signal: %s", sig.String())
		case <-done:
			c.logger.Debug("closing signal catching goroutine")
		}
		return nil
	}, func(err error) {
		close(done)
	})

	// print errors as they come in
	g.Add(func() error {
		for e := range errChan {
			c.logger.Error("error", "err", e)
		}
		return nil
	}, func(err error) {
		c.logger.Debug("closing error printing goroutine")
		close(errChan)
	})

	// read input from the user and send messages to the grpc server
	{
		var cReader cancelreader.CancelReader // bufio.Scanner.Scan() is a blocking call, and it's impossible to close os.Stdin, so linux epoll has to be used, a library for that is used here instead of implementing it myself
		var err error
		g.Add(func() error {
			cReader, err = cancelreader.NewReader(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to create cancel reader: %w", err)
			}
			scanner := bufio.NewScanner(cReader)
			c.logger.Info("enter message to send")
			for scanner.Scan() {
				line := scanner.Text()
				ctext, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				resp, err := pbClient.Send(ctext, &pb.SendRequest{Message: line})
				if err != nil {
					errChan <- fmt.Errorf("failed to send message: %w", err)
					cancel()
					continue
				}
				c.logger.Info("message sent", "response", resp)
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			return nil
		}, func(err error) {
			c.logger.Debug("closing input reading goroutine")
			cReader.Cancel()
		})
	}

	return g.Run()
}
