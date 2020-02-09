package server

import (
	pb "Polaris/src/rpc"
	"time"

	"github.com/processout/grpc-go-pool"
	"golang.org/x/net/context"
	//"google.golang.org/grpc"
)

type AcceptTxnBodyEvent struct {
	suc     bool
	err     error
	req     *pb.TxnBodyMessage
	res     *pb.Empty
	dstAddr string
	server  *Server
	timeout time.Duration
}

func (e *AcceptTxnBodyEvent) GetRPCError() error {
	return e.err
}

func (e *AcceptTxnBodyEvent) GetDstAddress() string {
	return e.dstAddr
}

func (e *AcceptTxnBodyEvent) Success() bool {
	return e.suc
}

func (e *AcceptTxnBodyEvent) Run(connPool *grpcpool.Pool) {
	//ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	//defer cancel()
	ctx := context.Background()
	logger.Debugf("Calling AccpetBody from server %v to server %v", e.server.ID, e.dstAddr)
	conn, err := connPool.Get(context.Background())
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection to pool to server %v, err %v", e.dstAddr, err.Error())
	}
	c := pb.NewPolarisAllClient(conn.ClientConn)
	e.res, e.err = c.AcceptTxnBody(ctx, e.req)
	if e.err == nil {
		e.suc = true
		//	return
	} else {
		logger.Errorf("calling acceptbody rpc error: %v", e.err)
	}
	//e.server.handleRPCError(e.err)
}

func NewAcceptTxnBodyEvent(msg *pb.TxnBodyMessage, timeout time.Duration, dstAddr string, server *Server) *AcceptTxnBodyEvent {
	return &AcceptTxnBodyEvent{
		suc:     false,
		err:     nil,
		req:     msg,
		res:     nil,
		timeout: timeout,
		server:  server,
		dstAddr: dstAddr,
	}
}
