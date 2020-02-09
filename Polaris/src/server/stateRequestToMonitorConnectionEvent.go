package server

import (
	pb "Polaris/src/rpc"
	"time"

	"golang.org/x/net/context"
	//"google.golang.org/grpc"

	"github.com/processout/grpc-go-pool"
)

type StateRequestToMonitorEvent struct {
	suc     bool
	err     error
	req     *pb.StateRequestMessage
	res     *pb.LeaderIdReply
	timeout time.Duration
	server  *Server
}

func (e *StateRequestToMonitorEvent) GetRPCError() error {
	return e.err
}

func (e *StateRequestToMonitorEvent) GetDstAddress() string {
	return e.server.GetLeader()
}

func (e *StateRequestToMonitorEvent) Success() bool {
	return e.suc
}

func (e *StateRequestToMonitorEvent) Run(connPool *grpcpool.Pool) {
	//ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	//defer cancel()
	ctx := context.Background()
	logger.Infof("Calling StateResponseToMonitor to server %v", e.server.ID)
	conn, err := connPool.Get(context.Background())
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection from pool state reqsponse %v error %v", e.server.GetLeader(), err.Error())
	}
	c := pb.NewPolarisAllClient(conn.ClientConn)
	e.res, e.err = c.StateRequestToMonitor(ctx, e.req)
	if e.err == nil {
		e.suc = true
		return
	}
	conn.Close()
	logger.Errorf("calling stateresponsetomonitor rpc error %v", e.err)
	if pool, handled := e.server.handleRPCError(e.err); handled {
		e.Run(pool)
	}
}

func NewStateRequestToMonitorEvent(msg *pb.StateRequestMessage, timeout time.Duration, server *Server) *StateRequestToMonitorEvent {
	return &StateRequestToMonitorEvent{
		suc:     false,
		err:     nil,
		req:     msg,
		res:     nil,
		timeout: timeout,
		server:  server,
	}

}
