package grpc

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/ethereum/go-ethereum/log"

	"github.com/dapplink-labs/dapplink-wallet-api/chaindispatcher"
	"github.com/dapplink-labs/dapplink-wallet-api/config"
	"github.com/dapplink-labs/dapplink-wallet-api/protobuf/walletapi"
)

const MaxReceivedMessageSize = 1024 * 1024 * 30000

type GrpcService struct {
	conf *config.Config
	walletapi.UnimplementedWalletApiGateWayServiceServer
	stopped atomic.Bool
}

func (s *GrpcService) Stop(ctx context.Context) error {
	s.stopped.Store(true)
	return nil
}

func (s *GrpcService) Stopped() bool {
	return s.stopped.Load()
}

func NewRpcService(conf *config.Config) (*GrpcService, error) {
	rpcService := &GrpcService{
		conf: conf,
	}
	return rpcService, nil
}

func (s *GrpcService) Start(ctx context.Context) error {
	go func(s *GrpcService) {

		addr := fmt.Sprintf("%s:%s", s.conf.RpcServer.Host, s.conf.RpcServer.Port)

		log.Info("rpc sever config", "addr", addr)

		dispatcher, err := chaindispatcher.NewChainDispatcher(s.conf)
		if err != nil {
			log.Error("new chain dispatcher fail", "err", err)
			return
		}

		gs := grpc.NewServer(
			grpc.MaxRecvMsgSize(MaxReceivedMessageSize),
			grpc.MaxSendMsgSize(MaxReceivedMessageSize),
			grpc.ChainUnaryInterceptor(dispatcher.Interceptor),
		)
		defer gs.GracefulStop()

		walletapi.RegisterWalletApiGateWayServiceServer(gs, dispatcher)

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Error("Could not start tcp listener. ")
			return
		}

		reflection.Register(gs)

		log.Info("Grpc info", "port", s.conf.RpcServer.Port, "address", listener.Addr())

		if err := gs.Serve(listener); err != nil {
			log.Error("Could not GRPC services")
		}
	}(s)
	return nil
}
