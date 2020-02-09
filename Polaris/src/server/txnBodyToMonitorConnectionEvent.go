package server

import (
	pb "Polaris/src/rpc"
	"time"

	"golang.org/x/net/context"
	//"google.golang.org/grpc"
	"github.com/processout/grpc-go-pool"
)

type TxnBodyToMonitorEvent struct {
	suc     bool
	err     error
	req     *pb.TxnBodyMessage
	res     *pb.LeaderIdReply
	server  *Server
	timeout time.Duration
}

func (e *TxnBodyToMonitorEvent) GetRPCError() error {
	return e.err
}

func (e *TxnBodyToMonitorEvent) GetDstAddress() string {
	return e.server.GetLeader()
}

func (e *TxnBodyToMonitorEvent) Success() bool {
	return e.suc
}

func (e *TxnBodyToMonitorEvent) Run(connPool *grpcpool.Pool) {
	//ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	//defer cancel()
	ctx := context.Background()
	logger.Debugf("Calling AccpetBody from server %v to server %v", e.server.ID, e.server.GetLeader())
	conn, err := connPool.Get(ctx)
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection from the pool acceptbody %v, error %v", e.server.GetLeader(), err.Error())

	}
	c := pb.NewPolarisAllClient(conn.ClientConn)
	e.res, e.err = c.TxnBodyToMonitor(ctx, e.req)
	if e.err == nil {
		e.suc = true
		return
	}
	conn.Close()
	logger.Errorf("calling acceptbody rpc error %v", e.err)
	if pool, isHandled := e.server.handleRPCError(e.err); isHandled {
		e.Run(pool)
	} else {
		logger.Warningf("Error is not handled %v", e.err)
	}
}

func NewTxnBodyToMonitorEvent(msg *pb.TxnBodyMessage, timeout time.Duration, server *Server) *TxnBodyToMonitorEvent {
	return &TxnBodyToMonitorEvent{
		suc:     false,
		err:     nil,
		req:     msg,
		res:     nil,
		timeout: timeout,
		server:  server,
	}
}
