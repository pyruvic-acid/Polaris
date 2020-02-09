package server

import (
	pb "Polaris/src/rpc"
	"time"

	"golang.org/x/net/context"
	//"google.golang.org/grpc"
	"github.com/processout/grpc-go-pool"
)

type TxnMsgToMonitorEvent struct {
	suc     bool
	err     error
	req     *pb.TransactionMessage
	res     *pb.LeaderIdReply
	server  *Server
	timeout time.Duration
}

func (e *TxnMsgToMonitorEvent) GetRPCError() error {
	return e.err
}

func (e *TxnMsgToMonitorEvent) GetDstAddress() string {
	return e.server.GetLeader()
}

func (e *TxnMsgToMonitorEvent) Success() bool {
	return e.suc
}

func (e *TxnMsgToMonitorEvent) Run(connPool *grpcpool.Pool) {
	//ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	//defer cancel()
	ctx := context.Background()
	logger.Debugf("Calling txn msg to monitor from server %v to server %v", e.server.ID, e.server.GetLeader())
	conn, err := connPool.Get(ctx)
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection from the pool txn msg to monitor %v, error %v", e.server.GetLeader(), err)
	}
	c := pb.NewPolarisAllClient(conn.ClientConn)
	e.res, e.err = c.TxnMessageToMonitor(ctx, e.req)
	if e.err == nil {
		e.suc = true
		return
	}
	logger.Errorf("Calling AcceptBody rpc error %v", e.err)
	conn.Close()
	if pool, handled := e.server.handleRPCError(e.err); handled {
		e.Run(pool)
	}
}

func NewTxnMsgToMonitorEvent(msg *pb.TransactionMessage, timeout time.Duration, server *Server) *TxnMsgToMonitorEvent {
	return &TxnMsgToMonitorEvent{
		suc:     false,
		err:     nil,
		req:     msg,
		res:     nil,
		timeout: timeout,
		server:  server,
	}
}
