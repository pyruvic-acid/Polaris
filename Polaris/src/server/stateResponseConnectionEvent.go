package server

import (
	pb "Polaris/src/rpc"
	"time"

	//tcpr3 "Polaris/src/tcpconn"
	//"bytes"
	//"encoding/gob"
	//"net"

	"golang.org/x/net/context"
	//"google.golang.org/grpc"
	"github.com/processout/grpc-go-pool"
)

type StateResponseEvent struct {
	suc      bool
	err      error
	req      *pb.StateResponseMessage
	res      *pb.Empty
	dstAddr  string
	serverId string
	timeout  time.Duration
}

func (e *StateResponseEvent) GetRPCError() error {
	return e.err
}

func (e *StateResponseEvent) GetDstAddress() string {
	return e.dstAddr
}

func (e *StateResponseEvent) Success() bool {
	return e.suc
}

//func (e *StateResponseEvent) Run(c pb.PolarisClient) {
func (e *StateResponseEvent) Run(connPool *grpcpool.Pool) {
	//ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	//defer cancel()
	ctx := context.Background()
	logger.Infof("Calling StateResponse to server %v", e.serverId)
	conn, err := connPool.Get(context.Background())
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection from pool state response %v error %v", e.dstAddr, err.Error())
	}
	c := pb.NewPolarisAllClient(conn.ClientConn)
	e.res, e.err = c.StateResponse(ctx, e.req)
	if e.err == nil {
		e.suc = true
	} else {
		logger.Errorf("calling stateresponse rpc error %v", e.err)
	}
}

//func (e *StateResponseEvent) Run(mgr *tcpr3.TCPManager, conn *net.TCPConn) {
//	// Query or Answer? -- Answer
//	mgr.EncodeAndSend(e, byte('A'), conn)
//}

func NewStateResponseEvent(msg *pb.StateResponseMessage, timeout time.Duration, dstAddr string) *StateResponseEvent {
	return &StateResponseEvent{
		suc:      false,
		err:      nil,
		req:      msg,
		res:      nil,
		timeout:  timeout,
		dstAddr:  dstAddr,
		serverId: msg.SenderId,
	}
}
