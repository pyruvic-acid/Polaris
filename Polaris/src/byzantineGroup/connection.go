package byzantineGroup

import (
	//"Polaris/src/common"
	//pb "Polaris/src/rpc"
	"github.com/processout/grpc-go-pool"
	"google.golang.org/grpc"
	//"google.golang.org/grpc/codes"
	//"time"
)

type ConnectionEvent interface {
	Run(c *grpcpool.Pool)
	//Run(c pb.PolarisClient)
	Success() bool
	//GetRPCError() error
	//GetDstAddress() string
}

type Connection struct {
	//dstServerAddrs []string             // ip:port  destination addr
	dstServerAddr string
	echan         chan ConnectionEvent // Channel for receiving events to process

	//connectionMap map[string]*grpcpool.Pool
	connectionPool *grpcpool.Pool
	//	dstServerAddr string
}

func (c *Connection) GetConnectionPool() *grpcpool.Pool {
	return c.connectionPool
}

func (c *Connection) createConnectionPool(poolSize int) {
	var factory grpcpool.Factory
	factory = func() (*grpc.ClientConn, error) {
		conn, err := grpc.Dial(c.dstServerAddr, grpc.WithInitialWindowSize(2000000), grpc.WithInitialConnWindowSize(2000000), grpc.WithInsecure())
		if err != nil {
			logger.Fatalf("Failed to start gRPC connection: %v", err)
		}
		logger.Infof("Connected to server at %s", c.dstServerAddr)
		return conn, err
	}
	pool, err := grpcpool.New(factory, poolSize, poolSize, 0)
	if err != nil {
		logger.Fatalf("Failed to create gRPC pool: %v", err)
	}
	c.connectionPool = pool
	// create connections
	//	for _, address := range c.dstServerAddrs {
	//
	//		conn, err := grpc.Dial(address, grpc.WithInsecure())
	//		logger.Debugf("create connect to %v", address)
	//
	//		if err != nil {
	//			logger.Fatalf("did not connect: %v", err)
	//		}
	//
	//		c.connectionMap[address] = conn
	//	}

	// set default leader
	//c.dstServerAddr = c.dstServerAddrs[0]
}

// Used by the server state thread to add events
// to this connection.
func (c *Connection) AddEvent(event ConnectionEvent) {
	c.echan <- event
}

func (c *Connection) run() {
	logger.Debugf("Creating connection to server %v", c.dstServerAddr)

	// Server address
	//address := fmt.Sprintf("%v:%v", c.ipPort.Ip, c.ipPort.Port)
	//c.createConnection()

	// Fetch events from the event channel until it receives a nil
	for {
		event := <-c.echan
		if event == nil {
			logger.Warningf("Get nil Event terminates the connection object")
			break // Terminates the connection object
		}
		// TODO: Need to verify that broken connections are reconnected
		//       automatically.
		//logger.Debug("Send request to server %v", c.leader)

		//conn := c.connectionMap[event.GetDstAddress()]

		//client := pb.NewPolarisClient(conn)

		//event.Run(client)
		go event.Run(c.connectionPool)

		// handling failure
		//	for !event.Success() {
		//		err := event.GetRPCError()
		//		errCode := grpc.Code(err)
		//		if errCode == common.NotLeaderErrorCode {
		//			addr := c.connectionMap[grpc.ErrorDesc(event.GetRPCError())]
		//			//newClient := pb.NewPolarisClient(addr)
		//			//event.Run(newClient)
		//			event.Run(addr)
		//		} else if errCode == common.StateResponseNotReady || errCode == codes.DeadlineExceeded {
		//			logger.Debugf("RPC error %v break", err)
		//			break
		//		} else {
		//			logger.Fatalf("RPC error %v", err)
		//		}
		//	}
	}
}

// TODO: Does the connection need to know the remoteNodeID?
//       For now, we assume it doesn't the server state
//       thread is responsible for tracking this information.
//func NewConnection(dstServerAddrs []string, qLen int32) *Connection {
func NewConnection(dstServerAddr string, qLen int32, poolSize int) *Connection {
	logger.Debug("new connection")
	c := &Connection{
		dstServerAddr: dstServerAddr,
		echan:         make(chan ConnectionEvent, qLen),
		//connectionMap:  make(map[string]*grpc.ClientConn),
		//connectionMap:  make(map[string]*grpc.ClientConn),
		connectionPool: nil,
	}
	c.createConnectionPool(poolSize)
	go c.run() // Start the connection thread
	return c
}
