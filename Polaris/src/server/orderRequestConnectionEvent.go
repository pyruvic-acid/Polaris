package server

import (
	//pb "Polaris/src/sbft-rpc"
	pb "Polaris/src/rpc"
	"time"

	"github.com/processout/grpc-go-pool"
	"golang.org/x/net/context"
	//"google.golang.org/grpc"
)

type OrderRequestEvent struct {
	suc      bool
	err      error
	req      *pb.SLRequestMessage
	res      *pb.LeaderIdReply
	serverId string
	dstAddr  string
	timeout  time.Duration
}

func (e *OrderRequestEvent) Success() bool {
	return e.suc
}

func (e *OrderRequestEvent) GetRPCError() error {
	return e.err
}

func (e *OrderRequestEvent) GetDstAddress() string {
	return e.dstAddr
}

//func (e *OrderRequestEvent) Run(c pb.PolarisClient) {
func (e *OrderRequestEvent) Run(connPool *grpcpool.Pool) {
	//ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	//defer cancel()
	ctx := context.Background()
	logger.Infof("Calling Order to server %v", e.serverId)
	conn, err := connPool.Get(ctx)
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection from pool for order request %v error %v", e.dstAddr, err.Error())
	}
	c := pb.NewPolarisClient(conn.ClientConn)
	e.res, e.err = c.OrderRequest(ctx, e.req)
	if e.err == nil {
		e.suc = true
		logger.Infof("Success %v of calling order request from server %v, error %v", e.suc, e.serverId, e.err)
	} else {
		logger.Errorf("calling order request rpc error %v", e.err)
	}

}

func NewOrderRequestEvent(req *pb.SLRequestMessage, timeout time.Duration, dstAddr string) *OrderRequestEvent {
	return &OrderRequestEvent{
		suc:      false,
		req:      req,
		timeout:  timeout,
		dstAddr:  dstAddr,
		serverId: req.SenderId,
	}
}
