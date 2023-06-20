package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/signal"

	"golang.org/x/exp/slog"

	pb "github.com/mwasilew2/chatter/gen"
	"github.com/oklog/run"
	"github.com/oklog/ulid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ChatBoardCmd struct {
	// cli options
	Addr string `help:"address to connect on" default:":8080"`

	// State
	id ulid.ULID

	// Dependencies
	logger *slog.Logger
}

func (b *ChatBoardCmd) Run(cmdCtx *cmdContext) error {
	b.logger = cmdCtx.Logger.With("component", "ChatBoardCmd")
	b.logger.Info("starting chat board", "addr", b.Addr)

	// set up grpc client
	conn, err := grpc.Dial(b.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial server: %w", err)
	}
	defer conn.Close()
	pbClient := pb.NewChatServerClient(conn)

	// generate ulid
	u, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to create a ulid for the board: %w", err)
	}
	b.id = u
	b.logger.Debug("generated ulid", "ulid", b.id.String())

	// run goroutines
	g := run.Group{}

	// listen for termination signals
	osSigChan := make(chan os.Signal, 1)
	signal.Notify(osSigChan, os.Kill, os.Interrupt)
	done := make(chan struct{})
	g.Add(func() error {
		select {
		case sig := <-osSigChan:
			b.logger.Debug("caught signal", "signal", sig.String())
			return fmt.Errorf("caught signal: %s", sig.String())
		case <-done:
			b.logger.Debug("closing signal catching goroutine")
		}
		return nil
	}, func(err error) {
		close(done)
	})

	// print incoming messages
	donePrinter := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g.Add(func() error {
		stream, err := pbClient.Receive(ctx, &pb.ReceiveRequest{
			ClientId: b.id.String(),
			LastId:   0,
		})
		if err != nil {
			return fmt.Errorf("failed to connect to stream: %w", err)
		}
		for {
			select {
			case <-stream.Context().Done():
				if err := stream.Context().Err(); err != nil {
					return fmt.Errorf("stream context error: %w", err)
				}
				return nil
			case <-donePrinter:
				b.logger.Debug("closed printer goroutine")
				return nil
			default:
				r, err := stream.Recv() // this is a blocking call, can be canceled by closing the relevant context
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("failed to receive message: %w", err)
				}
				b.logger.Info("message received", "message", r.Message)
			}
		}
		return nil
	}, func(err error) {
		b.logger.Debug("closing printer goroutine")
		close(donePrinter)
		cancel()
		b.logger.Debug("closed printer goroutine")
	})

	return g.Run()
}
