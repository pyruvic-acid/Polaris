package server

import (
	pb "Polaris/src/rpc"
	"time"

	//tcpr3 "Polaris/src/tcpconn"
	//	"bytes"
	//	"encoding/gob"
	//	"net"

	"golang.org/x/net/context"
	//"google.golang.org/grpc"
	"github.com/processout/grpc-go-pool"
)

type StateRequestEvent struct {
	suc      chan bool
	succ     bool
	err      error
	req      *pb.StateRequestMessage
	Res      *pb.StateResponseMessage
	emulator string
	serverId string
	timeout  time.Duration
}

func (e *StateRequestEvent) GetRPCError() error {
	return e.err
}

func (e *StateRequestEvent) GetDstAddress() string {
	return e.emulator
}

func (e *StateRequestEvent) Success() bool {
	return e.succ
}

func (e *StateRequestEvent) SuccessWait() bool {
	return <-e.suc
}

//func (e *StateRequestEvent) Run(c pb.PolarisClient) {
func (e *StateRequestEvent) Run(connPool *grpcpool.Pool) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()
	//ctx := context.Background()
	logger.Warningf("get connection from pool to server %v cycle %v, avaliable %v", e.emulator, e.req.CycleId, connPool.Available())
	conn, err := connPool.Get(context.Background())
	logger.Warningf("Calling StateRequest from server %v to emulator %v cycle %v",
		e.serverId, e.emulator, e.req.CycleId)
	defer conn.Close()
	if err != nil {
		logger.Fatalf("cannot get connection from the pool staterequest %v, error %v", e.emulator, err.Error())
	}
	c := pb.NewPolarisAllClient(conn.ClientConn)
	e.Res, e.err = c.StateRequest(ctx, e.req)
	logger.Warningf("Calling StateRequest get result emulator %v cycle %v", e.emulator, e.req.CycleId)
	// successfully get response with a timeout
	if e.err == nil {
		e.suc <- true
		e.succ = true
		logger.Debugf("Success %v of calling stateRequest error %v cycle %v", e.succ, e.err, e.req.CycleId)
	} else {
		e.suc <- false
		e.succ = false
		logger.Errorf("Calling staterequest rpc error %v", e.err)
	}
	logger.Warningf("Calling StateRequest finish to emulator %v cycle %v", e.emulator, e.req.CycleId)
}

//func (e *StateRequestEvent) Run(mgr *tcpr3.TCPManager, conn *net.TCPConn) {
//	// Query or Answer? -- Query
//	mgr.EncodeAndSend(e, byte('Q'), conn)
//}

func NewStateRequestEvent(req *pb.StateRequestMessage, timeout time.Duration, dstAddr string) *StateRequestEvent {
	return &StateRequestEvent{
		suc:      make(chan bool),
		succ:     false,
		err:      nil,
		req:      req,
		Res:      nil,
		serverId: req.SenderId,
		timeout:  timeout,
		emulator: dstAddr,
	}
}
