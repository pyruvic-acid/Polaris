package server

import (
	pb "Polaris/src/rpc"
	"time"

	"golang.org/x/net/context"
	//"google.golang.org/grpc"
	"github.com/processout/grpc-go-pool"
)

type StateResponseToMonitorEvent struct {
	suc     bool
	err     error
	req     *pb.StateResponseMessage
	res     *pb.LeaderIdReply
	server  *Server
	timeout time.Duration
}

func (e *StateResponseToMonitorEvent) GetRPCError() error {
	return e.err
}

func (e *StateResponseToMonitorEvent) GetDstAddress() string {
	return e.server.GetLeader()
}

func (e *StateResponseToMonitorEvent) Success() bool {
	return e.suc
}

//func (e *StateResponseToMonitorEvent) Run(c pb.PolarisClient) {
func (e *StateResponseToMonitorEvent) Run(connPool *grpcpool.Pool) {
	//ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	//defer cancel()
	ctx := context.Background()
	logger.Infof("Calling StateResponseToMonitor from server %v to server %v", e.server.ID, e.server.GetLeader())
	conn, err := connPool.Get(context.Background())
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection from the pool state response to monitor %v, error %v", e.server.GetLeader(), err.Error())
	}
	c := pb.NewPolarisAllClient(conn.ClientConn)
	e.res, e.err = c.StateResponseToMonitor(ctx, e.req)
	if e.err == nil {
		e.suc = true
		return
	}
	conn.Close()
	logger.Errorf("calling stateResponseToMonitor rpc error %v", e.err)
	if pool, handled := e.server.handleRPCError(e.err); handled {
		e.Run(pool)
	}
}

func NewStateResponseToMonitorEvent(msg *pb.StateResponseMessage, timeout time.Duration, server *Server) *StateResponseToMonitorEvent {
	return &StateResponseToMonitorEvent{
		suc:     false,
		err:     nil,
		req:     msg,
		res:     nil,
		timeout: timeout,
		server:  server,
	}

}
