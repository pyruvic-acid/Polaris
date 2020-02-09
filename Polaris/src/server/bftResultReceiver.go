package server

import (
	"context"
	//"flag"

	pb "Polaris/src/rpc"
	//sbft "Polaris/src/sbft-rpc"
	"io"
	"time"

	"google.golang.org/grpc"
	//"google.golang.org/grpc/credentials"
	//"fmt"
)

type BftResultReceiver struct {
	replicaAddress string
	result         chan *pb.BftReply
}

func NewBftResultReceiver(replicaAddress string) *BftResultReceiver {
	receiver := &BftResultReceiver{
		replicaAddress: replicaAddress,
		result:         make(chan *pb.BftReply, 1000),
	}
	return receiver
}

func (r *BftResultReceiver) getResult(client pb.PolarisClient, lr *pb.BftRequest) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.GetOrder(ctx, lr)
	if err != nil {
		logger.Fatalf("%v.GetOrder() = _, %v", client, err)
	}

	for {
		block, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			logger.Fatalf("%v.GetOrder() = _, recv %v", client, err)
		}
		senderIds := make([]string, 0)
		for _, message := range block.MessageList {
			senderIds = append(senderIds, message.SenderId)
		}
		logger.Warningf("receive block: sequence %v num of requests %v from servers %v", block.Sequence, len(block.MessageList), senderIds)
		r.result <- block
	}
}

func (r *BftResultReceiver) Start() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, r.replicaAddress, grpc.WithBlock(), grpc.WithInsecure())
	for err != nil {
		// when polaris-server starts, the sbft replica does not start
		// sbft replica need to run on the monitor server
		// so sbft replica starts when monitor is selected
		// therefore, we need to try agian
		logger.Warningf("fail to dial: %v; Try again", err)
		// ctx will expired when times out
		// TODO: check: should create ctx again?
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err = grpc.DialContext(ctx, r.replicaAddress, grpc.WithBlock(), grpc.WithInsecure())

	}
	defer conn.Close()
	logger.Debugf("Connected to sbft replica %v", r.replicaAddress)

	client := pb.NewPolarisClient(conn)

	for {
		r.getResult(client, &pb.BftRequest{})
	}
}

// Get SBFT result for the cycle
func (r *BftResultReceiver) GetNextCycle() (*pb.BftResult, []string) {
	reply := <-r.result

	senderIds := make([]string, 0)
	txnHeaderList := make([]*pb.TransactionHeader, 0)

	for _, message := range reply.MessageList {
		senderIds = append(senderIds, message.SenderId)
		if message.TxnHeaderBlock == nil {
			logger.Error("Error: message.TxnHeaderBlock is nil!")
		}

		txnHeaderList = append(txnHeaderList, message.TxnHeaderBlock.TxnHeaderList...)
	}

	result := &pb.BftResult{
		Sequence:               reply.Sequence,
		View:                   reply.View,
		Nonce:                  reply.Nonce,
		LastCycle:              reply.LastCycle,
		CurrentCycle:           reply.CurrentCycle,
		CommitProof:            reply.CommitProof,
		ReplaceMsgListWithHash: false,
		Hash:         nil,
		MessageList:  reply.MessageList,
		BodyAttached: false,
		FullTxnList:  nil,
	}

	return result, senderIds
}
