package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/muesli/cancelreader"
	"github.com/oklog/run"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc"

	pb "github.com/mwasilew2/chatter/gen"
)

type chatClient struct {
	logger *log.Logger
}

func NewChatClient(logger *log.Logger) *chatClient {
	return &chatClient{logger: logger}
}

func (s *chatClient) run(ctx *cli.Context) error {
	level.Debug(*s.logger).Log("msg", "starting client")

	// set up grpc client
	port := ctx.Int("port")
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}
	defer conn.Close()
	c := pb.NewChatServerClient(conn)

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
			level.Debug(*s.logger).Log("msg", "caught signal", "signal", sig.String())
			return fmt.Errorf("received signal: %s", sig.String())
		case <-done:
			level.Debug(*s.logger).Log("msg", "closing signal catching goroutine")
		}
		return nil
	}, func(err error) {
		close(done)
	})

	// print errors as they come in
	g.Add(func() error {
		for e := range errChan {
			level.Error(*s.logger).Log("err", fmt.Errorf("error %w", e))
		}
		return nil
	}, func(err error) {
		level.Debug(*s.logger).Log("msg", "closing error printing goroutine")
		close(errChan)
	})

	// read input from the user and send messages to the grpc server
	{
		var r cancelreader.CancelReader // bufio.Scanner.Scan() is a blocking call and it's impossible to close os.Stdin, so linux epoll has to be used, a library for that is used here instead of implementing it myself
		var err error
		g.Add(func() error {
			r, err = cancelreader.NewReader(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to create cancel reader: %w", err)
			}
			scanner := bufio.NewScanner(r)
			level.Info(*s.logger).Log("msg", "enter message to send")
			for scanner.Scan() {
				line := scanner.Text()
				ctext, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				r, err := c.Send(ctext, &pb.SendRequest{Message: line})
				if err != nil {
					errChan <- fmt.Errorf("failed to send message: %w", err)
					cancel()
					continue
				}
				level.Debug(*s.logger).Log("msg", "message sent", "response", r)
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			return nil
		}, func(err error) {
			level.Debug(*s.logger).Log("msg", "closing input reading goroutine")
			r.Cancel()
		})
	}

	return g.Run()
}
