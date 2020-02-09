package server

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"Polaris/src/common"
	pb "Polaris/src/rpc"

	//sbft "Polaris/src/sbft-rpc"

	"google.golang.org/grpc"
)

type BftResultVerifier struct {
	verifierAddress string

	client pb.PolarisClient
}

func NewBftResultVerifier(addr string) *BftResultVerifier {
	v := &BftResultVerifier{
		verifierAddress: addr,
	}

	conn, err := grpc.Dial(addr, grpc.WithBlock(), grpc.WithInsecure())

	if err != nil {
		logger.Debugf("cannot connenct verification server %v", err)
	}

	logger.Debugf("verifier server %v connected", addr)

	v.client = pb.NewPolarisClient(conn)

	return v
}

func (v *BftResultVerifier) verifyRequest(stateRequest *pb.StateRequestMessage) bool {
	// name-removed: Warning -- all machines must be little endian!
	lastCycle := binary.LittleEndian.Uint32(stateRequest.Hash[0:4])
	currentCycle := binary.LittleEndian.Uint32(stateRequest.Hash[4:8])
	if stateRequest.CycleId <= int32(lastCycle) || stateRequest.CycleId > int32(currentCycle) {
		logger.Errorf("%d is not within (%d, %d]", stateRequest.CycleId, int32(lastCycle), int32(currentCycle))
		return false
	}

	digest := fmt.Sprintf("%x", stateRequest.Hash)

	data := &pb.VerifyRequest{
		View:          stateRequest.View,
		Sequence:      stateRequest.Sequence,
		CommitProof:   stateRequest.Proof,
		RequestDigest: digest,
	}

	//logger.Debugf("Verifying Request %v for state response %v", data, stateResponse)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err1 := v.client.Verify(ctx, data)

	if err1 != nil {
		logger.Fatalf("%v.Verify(_) = _, %v: ", v.client, err1)
	}

	if !result.Status {
		fmt.Printf("verify result: %v for request %v\n", result.Status, data)
	}

	logger.Debugf("verify result: %v for state request cycle %v from server %v", result.Status, stateRequest.CycleId, stateRequest.SenderId)
	return result.Status
}

func (v *BftResultVerifier) verifyResponse(stateResponse *pb.StateResponseMessage) bool {
	common.AssertTrue(stateResponse.Result.Hash != nil, "Hash should have been calculated!", logger)

	hash := stateResponse.Result.Hash
	digest := fmt.Sprintf("%x", hash)

	data := &pb.VerifyRequest{
		View:          stateResponse.Result.View,
		Sequence:      stateResponse.Result.Sequence,
		CommitProof:   stateResponse.Result.CommitProof,
		RequestDigest: digest,
	}

	//logger.Debugf("Verifying Request %v for state response %v", data, stateResponse)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err1 := v.client.Verify(ctx, data)

	if err1 != nil {
		logger.Fatalf("%v.Verify(_) = _, %v: ", v.client, err1)
	}

	if !result.Status {
		fmt.Printf("verify result: %v for request %v\n", result.Status, data)
	}

	logger.Debugf("verify result: %v for state response cycle %v from server %v", result.Status, stateResponse.CycleId, stateResponse.SenderId)
	return result.Status
}
