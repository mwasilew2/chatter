package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"

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
	subscribers sync.Map
}

type subscriber struct {
	stream   pb.ChatServer_ReceiveServer
	finished chan<- struct{}
}

var (
	messagesChannel = make(chan string, 10)
)

func (s *chatServer) Send(ctx context.Context, request *pb.SendRequest) (*pb.SendResponse, error) {
	level.Info(s.logger).Log("msg", "received message", "message", request.Message)
	messagesChannel <- request.Message
	return &pb.SendResponse{Status: 0}, nil
}

func (s *chatServer) Receive(request *pb.ReceiveRequest, server pb.ChatServer_ReceiveServer) error {
	level.Debug(s.logger).Log("msg", "subscribing client", "clientId", request.ClientId)
	f := make(chan struct{})
	s.subscribers.Store(request.ClientId, subscriber{stream: server, finished: f})

	for {
		select {
		case <-f:
			level.Debug(s.logger).Log("msg", "closing stream for client", "clientId", request.ClientId)
			return nil
		case <-server.Context().Done():
			level.Debug(s.logger).Log("msg", "client disconnected", "clientId", request.ClientId)
			return nil
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
		srv.Stop()
		level.Debug(s.logger).Log("msg", "grpc server stopped")
	})

	// run the broadcast goroutine which sends messages to all subscribers
	doneBroadcast := make(chan struct{})
	g.Add(func() error {
		for {
			select {
			case msg := <-messagesChannel:
				s.subscribers.Range(func(key, value interface{}) bool {
					id, ok := key.(string)
					if !ok {
						level.Error(s.logger).Log("msg", "error casting key to string", "key", key)
						return false
					}
					sub, ok := value.(subscriber)
					if !ok {
						level.Error(s.logger).Log("msg", "error casting value to subscriber", "value", value)
						return false
					}
					if err := sub.stream.Send(&pb.ReceiveResponse{Message: msg}); err != nil {
						level.Error(s.logger).Log("msg", "error sending message to client", "clientId", id, "err", err)
						sub.finished <- struct{}{}
						s.subscribers.Delete(key)
					}
					return true
				})
			case <-doneBroadcast:
				level.Debug(s.logger).Log("msg", "broadcast goroutine stopped")
				return nil
			}
		}
		return nil
	}, func(err error) {
		level.Debug(s.logger).Log("msg", "shutting down broadcast goroutine")
		close(doneBroadcast)
	})

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

	return g.Run()
}
