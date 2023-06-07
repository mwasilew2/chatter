package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	pb "github.com/mwasilew2/chatter/gen"
	"github.com/oklog/run"
	"github.com/oklog/ulid"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type chatBoard struct {
	logger log.Logger

	id ulid.ULID
}

func (s *chatBoard) run(ctx *cli.Context) error {
	level.Debug(s.logger).Log("msg", "initializing board")

	// set up grpc client
	port := ctx.Int("port")
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}
	defer conn.Close()
	c := pb.NewChatServerClient(conn)

	// generate ulid
	u, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to create a ulid for the board: %w", err)
	}
	s.id = u
	level.Debug(s.logger).Log("msg", "generated ulid", "ulid", s.id.String())

	// run goroutines
	g := run.Group{}

	// listen for termination signals
	osSigChan := make(chan os.Signal, 1)
	signal.Notify(osSigChan, os.Kill, os.Interrupt)
	done := make(chan struct{})
	g.Add(func() error {
		select {
		case sig := <-osSigChan:
			level.Debug(s.logger).Log("msg", "caught signal", "signal", sig.String())
			return fmt.Errorf("caught signal: %s", sig.String())
		case <-done:
			level.Debug(s.logger).Log("msg", "closing signal catching goroutine")
		}
		return nil
	}, func(err error) {
		close(done)
	})

	// print incoming messages
	donePrinter := make(chan struct{})
	g.Add(func() error {
		stream, err := c.Receive(context.Background(), &pb.ReceiveRequest{
			ClientId: s.id.String(),
			LastId:   0,
		})
		if err != nil {
			return fmt.Errorf("failed to connect to stream: %w", err)
		}
		for {
			r, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("failed to receive message: %w", err)
			}
			level.Info(s.logger).Log("msg", "message received", "message", r.Message)
		}
		return nil
	}, func(err error) {
		// TODO: this doesn't actually terminate the goroutine
		level.Debug(s.logger).Log("msg", "closing printer goroutine")
		close(donePrinter)
	})

	return g.Run()
}
