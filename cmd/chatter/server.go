package main

import (
	"context"
	"fmt"
	"net"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	pb "github.com/mwasilew2/chatter/gen"
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
	//TODO implement me
	panic("implement me")
}

func (s *chatServer) run(ctx *cli.Context) error {
	level.Info(s.logger).Log("msg", "running server")
	port := ctx.Int("port")

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	srv := grpc.NewServer()
	pb.RegisterChatServerServer(srv, s)
	level.Info(s.logger).Log("msg", "server listening", "port", port)
	if err := srv.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}
