package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	pb "github.com/mwasilew2/chatter/gen"
	"github.com/oklog/run"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
)

type chatServer struct {
	logger log.Logger
	pb.UnimplementedChatServerServer
}

var (
	messagesBuffer = make([]string, 0)
)

func (s *chatServer) Send(ctx context.Context, request *pb.SendRequest) (*pb.SendResponse, error) {
	level.Info(s.logger).Log("msg", "received message", "message", request.Message)
	messagesBuffer = append(messagesBuffer, request.Message)
	return &pb.SendResponse{Status: 0}, nil
}

func (s *chatServer) Receive(request *pb.ReceiveRequest, server pb.ChatServer_ReceiveServer) error {
	level.Debug(s.logger).Log("msg", "received request for messages")
	for _, message := range messagesBuffer {
		if err := server.Send(&pb.ReceiveResponse{Message: message}); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	}
	return nil

}

func (s *chatServer) run(ctx *cli.Context) error {
	level.Debug(s.logger).Log("msg", "initializing grpc server")

	// run goroutines
	g := run.Group{}

	// start grpc server
	port := ctx.Int("port")
	var srv *grpc.Server
	g.Add(func() error {
		level.Debug(s.logger).Log("msg", "starting grpc server")

		lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			return fmt.Errorf("failed to listen: %w", err)
		}
		srv = grpc.NewServer()
		pb.RegisterChatServerServer(srv, s)
		level.Info(s.logger).Log("msg", "server listening", "port", port)
		return srv.Serve(lis)
	}, func(err error) {
		level.Debug(s.logger).Log("msg", "shutting down grpc server")
		srv.GracefulStop()
	})

	// listen for termination signals
	osSigChan := make(chan os.Signal, 1)
	signal.Notify(osSigChan, os.Kill, os.Interrupt)
	done := make(chan struct{})
	g.Add(func() error {
		select {
		case sig := <-osSigChan:
			level.Debug(s.logger).Log("msg", "caught signal", "singal", sig.String())
			return fmt.Errorf("caught signal: %s", sig.String())
		case <-done:
			level.Debug(s.logger).Log("msg", "closing signal catching goroutine")
		}
		return nil
	}, func(err error) {
		close(done)
	})

	return g.Run()
}
