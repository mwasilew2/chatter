package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"

	"golang.org/x/exp/slog"

	pb "github.com/mwasilew2/chatter/gen"
	"github.com/oklog/run"
	"google.golang.org/grpc"
)

type ChatServerCmd struct {
	// cli options
	Addr string `help:"address to listen on" default:":8080"`

	// State
	subscribers     sync.Map
	messagesChannel chan string

	// Dependencies
	logger *slog.Logger

	// Interfaces
	pb.UnimplementedChatServerServer
}

type subscriber struct {
	stream          pb.ChatServer_ReceiveServer
	finishedChannel chan<- struct{}
}

func (s *ChatServerCmd) Run(cmdCtx *cmdContext) error {
	s.logger = cmdCtx.Logger.With("component", "ChatServerCmd")
	s.logger.Info("starting chat server", "addr", s.Addr)
	s.messagesChannel = make(chan string, 10)

	// run goroutines
	g := run.Group{}

	// start grpc server
	var srv *grpc.Server
	g.Add(func() error {
		s.logger.Debug("starting grpc server")

		lis, err := net.Listen("tcp", s.Addr)
		if err != nil {
			return fmt.Errorf("failed to listen: %w", err)
		}
		srv = grpc.NewServer()
		pb.RegisterChatServerServer(srv, s)
		s.logger.Info("server listening", "address", s.Addr)
		return srv.Serve(lis)
	}, func(err error) {
		s.logger.Debug("shutting down grpc server")
		srv.Stop()
		s.logger.Debug("grpc server stopped")
	})

	// run the broadcast goroutine which sends messages to all subscribers
	doneBroadcast := make(chan struct{})
	g.Add(func() error {
		for {
			select {
			case msg := <-s.messagesChannel:
				s.broadcastMessage(msg)
			case <-doneBroadcast:
				s.logger.Debug("broadcast goroutine stopped")
				return nil
			}
		}
	}, func(err error) {
		s.logger.Debug("shutting down broadcast goroutine")
		close(doneBroadcast)
	})

	// listen for termination signals
	osSigChan := make(chan os.Signal, 1)
	signal.Notify(osSigChan, os.Kill, os.Interrupt)
	done := make(chan struct{})
	g.Add(func() error {
		select {
		case sig := <-osSigChan:
			s.logger.Debug("caught signal", "signal", sig.String())
			return fmt.Errorf("caught signal: %s", sig.String())
		case <-done:
			s.logger.Debug("signal catching goroutine stopped")
		}
		return nil
	}, func(err error) {
		close(done)
	})

	return g.Run()
}

func (s *ChatServerCmd) broadcastMessage(msg string) {
	s.subscribers.Range(func(key, value interface{}) bool {
		id, ok := key.(string)
		if !ok {
			s.logger.Error("error casting key to string", "key", key)
			return false
		}
		sub, ok := value.(subscriber)
		if !ok {
			s.logger.Error("error casting value to subscriber", "value", value)
			return false
		}
		if err := sub.stream.Send(&pb.ReceiveResponse{Message: msg}); err != nil {
			s.logger.Error("error sending message to client", "clientId", id, "err", err)
			sub.finishedChannel <- struct{}{}
			s.subscribers.Delete(key)
			return false
		}
		return true
	})
}

func (s *ChatServerCmd) Send(ctx context.Context, request *pb.SendRequest) (*pb.SendResponse, error) {
	s.logger.Info("received message", "message", request.Message)
	s.messagesChannel <- request.Message
	return &pb.SendResponse{Status: 0}, nil
}

func (s *ChatServerCmd) Receive(request *pb.ReceiveRequest, server pb.ChatServer_ReceiveServer) error {
	s.logger.Debug("received subscription request", "clientId", request.ClientId)
	f := make(chan struct{})
	s.subscribers.Store(request.ClientId, subscriber{stream: server, finishedChannel: f})

	for {
		select {
		case <-f:
			s.logger.Debug("closing stream for client", "clientId", request.ClientId)
			return nil
		case <-server.Context().Done():
			s.logger.Debug("client disconnected", "clientId", request.ClientId)
			s.subscribers.Delete(request.ClientId)
			return nil
		}
	}
}
