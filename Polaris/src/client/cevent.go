package client

import (
	pb "Polaris/src/rpc"
	"time"

	grpcpool "github.com/processout/grpc-go-pool"
	"golang.org/x/net/context"
	//"google.golang.org/grpc"
)

type SendTxnEvent struct {
	suc        bool
	err        error
	req        *pb.TransactionMessage
	res        *pb.Empty
	clientId   string
	dstAddress string
	timeout    time.Duration
}

func (e *SendTxnEvent) Run(connPool *grpcpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()
	//logger.Debugf("send txn getting conn from pool, available %v", connPool.Available())
	conn, err := connPool.Get(context.Background())
	//logger.Debugf("send txn get conn from pool, available %v", connPool.Available())
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection from the pool client send txn %v", e.dstAddress)
	}
	//logger.Debugf("Calling Accept Txn from client %v", e.clientId)
	c := pb.NewPolarisAllClient(conn.ClientConn)
	e.res, e.err = c.AcceptTransaction(ctx, e.req)

	if e.err == nil {
		e.suc = true
	}
	//	logger.Debugf("closing the conn")
	//	err = conn.Close()
	//
	//	if err != nil {
	//		logger.Fatalf("cannot close the conn error %v", err)
	//	}

	//logger.Debugf("Success %v of calling Accept Txn from client %v, err %v",
	//	e.suc, e.clientId, e.err)
}

func (e *SendTxnEvent) Success() bool {
	return e.suc
}

func (e *SendTxnEvent) GetRPCError() error {
	return e.err
}

func (e *SendTxnEvent) GetDstAddress() string {
	return e.dstAddress
}

func NewSendTxnEvent(req *pb.TransactionMessage, timeout time.Duration, dstAddr string) *SendTxnEvent {
	//logger.Debugf("new send txn event")
	return &SendTxnEvent{
		suc:        false,
		req:        req,
		timeout:    timeout,
		dstAddress: dstAddr,
		clientId:   req.SenderId,
	}
}

type GetCycleEvent struct {
	suc        bool
	req        *pb.RequestCycleMessage
	res        *pb.CycleInfo
	err        error
	dstAddress string
	timeout    time.Duration
	wait       chan bool
}

func NewGetCycleEvent(req *pb.RequestCycleMessage, timeout time.Duration, dstAddr string) *GetCycleEvent {
	return &GetCycleEvent{
		suc:        false,
		req:        req,
		res:        nil,
		err:        nil,
		timeout:    timeout,
		dstAddress: dstAddr,
		wait:       make(chan bool),
	}
}

func (e *GetCycleEvent) Success() bool {
	return e.suc
}

func (e *GetCycleEvent) GetRPCError() error {
	return e.err
}

func (e *GetCycleEvent) GetDstAddress() string {
	return e.dstAddress
}

func (e *GetCycleEvent) WaitAndGetResponse() *pb.CycleInfo {
	<-e.wait
	return e.res
}

func (e *GetCycleEvent) Run(connPool *grpcpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()
	//logger.Debugf("cycle result getting conn from pool available %v", connPool.Available())
	conn, err := connPool.Get(context.Background())
	//logger.Debugf("cycle result get conn from pool available %v", connPool.Available())
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection from the pool client get cycle %v", e.dstAddress)

	}
	//logger.Debugf("Calling Get Cycle event for cycle %v", e.req.CycleId)
	c := pb.NewPolarisAllClient(conn.ClientConn)
	e.res, e.err = c.GetCycle(ctx, e.req)

	if e.err == nil {
		e.suc = true
	} else {
		logger.Errorf("Fail get cycle result for cycle %v err %v", e.req.CycleId, e.err)
	}
	e.wait <- true
	//conn.Close()
	//logger.Debugf("closing the conn")
	//err = conn.Close()

	//if err != nil {
	//	logger.Fatalf("cannot close the conn error %v", err)
	//}
}
