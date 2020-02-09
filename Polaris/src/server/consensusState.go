package server

import (
	pb "Polaris/src/rpc"
	"Polaris/src/util"
	"runtime"

	//"fmt"
	"sort"
	//"time"
)

// we have consensus state for each cycle
// In a consensus state, it store the state response from different BG
// Check if there are pending state requests
// Check if there are get cycle info requests
type ConsensusState struct {
	cycleId int

	// channal for operations that read and modify
	// the fields in this consensus state
	accessOp chan accessConsensusStateOp

	// list of BG StateResponse (unordered)
	bgStateResponse []*pb.StateResponseMessage

	// store ordered state response by random number
	orderedStateResponse []*pb.StateResponseMessage

	// received bg response
	bgReceiveNum int

	// list of wait cycle complete requests
	// the cycle completes when it receives all the state responses from all the BGs
	// when it receives all the state response, it will reply the cycle result
	// back to servers in this list
	waitCompleteOp []*WaitCycleComplete

	justWaitCompleteOp []*JustWaitCycleComplete

	waitOwnStateResponse []*GetAndWaitOwnBGStateResponseOp

	// list of pending state requests
	// when the server does not complete the cycle, and receive a state request
	// it store the request this list
	pendingStateRequest []*GetOrWaitBGStateResponse

	// pointer to the server
	server *Server
}

func NewConsensusState(bgNum int, cycleId int, server *Server) *ConsensusState {
	cs := &ConsensusState{
		cycleId:              cycleId,
		bgStateResponse:      make([]*pb.StateResponseMessage, bgNum),
		accessOp:             make(chan accessConsensusStateOp, 5000),
		waitCompleteOp:       make([]*WaitCycleComplete, 0),
		justWaitCompleteOp:   make([]*JustWaitCycleComplete, 0),
		waitOwnStateResponse: make([]*GetAndWaitOwnBGStateResponseOp, 0),
		pendingStateRequest:  make([]*GetOrWaitBGStateResponse, 0),
		bgReceiveNum:         0,
		server:               server,
	}

	// single thread add the state response
	// and keep check of if the cycle complete
	go cs.run()

	return cs
}

// single thread process all the read and modify operations to this consensus state
func (c *ConsensusState) run() {
	for {
		select {
		case op := <-c.accessOp:
			logger.Debugf("cycle %v execute op", c.cycleId)
			op.Execute()
		}
	}
}

// add operations to the channel
// return immediatelly the caller before execute the operation
func (c *ConsensusState) AddOP(op accessConsensusStateOp) {
	logger.Debugf("Add operation to cycle %v", c.cycleId)
	c.accessOp <- op
}

// add operations to the channel
// return to the caller after the operation is executed
func (c *ConsensusState) AddOPandWait(op accessConsensusStateOp) bool {
	logger.Debugf("Add operation to cycle %v and wait", c.cycleId)
	c.accessOp <- op
	return op.BlockOwner()
}

// add to the list of waiting cycle completet
func (c *ConsensusState) AddWaitCompleteOp(op *WaitCycleComplete) {
	c.waitCompleteOp = append(c.waitCompleteOp, op)
}

// add to the list of waiting cycle completet
func (c *ConsensusState) AddJustWaitCompleteOp(op *JustWaitCycleComplete) {
	c.justWaitCompleteOp = append(c.justWaitCompleteOp, op)
}

// add to the list of pending state request
func (c *ConsensusState) AddPendingStateRequestOp(op *GetOrWaitBGStateResponse) {
	c.pendingStateRequest = append(c.pendingStateRequest, op)
}

// check if the state response of BG whose ID is bgId is received
func (c *ConsensusState) IsBGStateExist(bgId int) bool {
	rep := c.bgStateResponse[bgId]
	return rep != nil
}

// when receive a state response,
// add to the bgStateResponse list if it is not added before
func (c *ConsensusState) AddBGState(sp *pb.StateResponseMessage) {
	if c.bgStateResponse[sp.BgId] == nil {
		c.bgReceiveNum++
		c.bgStateResponse[sp.BgId] = sp

		// if it is own state response,
		// send the response back to servers who are waiting for state response
		if c.server.BGID == int(sp.BgId) {
			c.wakeUpWaitOwnStateResponse(sp)
			c.wakeUpPendingStateRequest(sp)
		}
		logger.Warningf("Add state reponse for bg %v cycle %v from server %v, msg sender %v, txn num %v", sp.BgId, sp.CycleId, sp.SenderId, sp.SenderId, util.CountNumberOfTxnInBftResult(sp.Result))

		// if the cycle is complete (receives all the state response from all the BGs)
		// send the cycle result back to clients who are waiting for the cycle completion
		if c.IsConsensus() {
			// order the list of state response by random number
			c.orderTxn()
			// send result back
			c.wakeUpJustWaitCompleteOp()
			c.wakeUpWaitCompleteOp()
			// print the log for testing
			c.printOrderedTxn()
		}
	} else {
		logger.Debugf("State respnse for cycle %v bgId %v is already added",
			sp.CycleId, sp.BgId)
	}
}

func (c *ConsensusState) AddWaitOwnStateResponse(o *GetAndWaitOwnBGStateResponseOp) {
	logger.Debugf("add wait own state response op for cycle %v", c.cycleId)
	c.waitOwnStateResponse = append(c.waitOwnStateResponse, o)
	//if c.waitOwnStateResponse != nil {
	//	logger.Fatalf("should only wait for own state response once cycle %v", c.cycleId)
	//}

	//c.waitOwnStateResponse = o
}

func (c *ConsensusState) wakeUpWaitOwnStateResponse(sp *pb.StateResponseMessage) {
	// if there is no operation waiting for own state response, do nothing
	for _, op := range c.waitOwnStateResponse {
		op.result = sp
		op.wait <- true
	}

	c.waitOwnStateResponse = make([]*GetAndWaitOwnBGStateResponseOp, 0)
	//	if c.waitOwnStateResponse == nil {
	//		return
	//	}
	//
	//	c.waitOwnStateResponse.result = sp
	//	c.waitOwnStateResponse.wait <- true
	//	c.waitOwnStateResponse = nil
}

// when the server complete this cycle, then send its state response back to servers who
// are waiting for the state response
func (c *ConsensusState) wakeUpPendingStateRequest(sp *pb.StateResponseMessage) {
	for _, op := range c.pendingStateRequest {
		logger.Debugf("Send state response to server %v who is pending for cycle %v bg %v",
			op.stateRequest.SenderId, op.stateRequest.CycleId, sp.BgId)

		//conn := c.server.BGsInfo[op.stateRequest.BgId].GetConnection(op.stateRequest.SenderAddr)
		sp.SenderId = c.server.ID

		tcpAddr := c.server.serverTCPAddrMap[op.stateRequest.SenderAddr]
		c.server.tcpManager.EncodeAndSend(tcpAddr, 2, sp)
	}

	// clear the pending list
	c.pendingStateRequest = make([]*GetOrWaitBGStateResponse, 0)
}

// when the server receives all the state responses from all the BGs
// send the cycle result back to the clients who are waiting for the cycle completion
func (c *ConsensusState) wakeUpJustWaitCompleteOp() {
	for _, op := range c.justWaitCompleteOp {
		op.wait <- true
	}
	// clean up waiting list
	c.justWaitCompleteOp = make([]*JustWaitCycleComplete, 0)
}

// when the server receives all the state responses from all the BGs
// send the cycle result back to the clients who are waiting for the cycle completion
func (c *ConsensusState) wakeUpWaitCompleteOp() {
	for _, op := range c.waitCompleteOp {
		op.result = c.GetOrderedBGStateResponse()
		op.wait <- true
	}
	// clean up waiting list
	c.waitCompleteOp = make([]*WaitCycleComplete, 0)
}

// check if the cycle completes
// by checking the number of state response received from BGs
func (c *ConsensusState) IsConsensus() bool {
	return c.bgReceiveNum == len(c.bgStateResponse)
}

// returns the state responses by bgId
func (c *ConsensusState) GetBGStateResponseById(bgId int) *pb.StateResponseMessage {
	return c.bgStateResponse[bgId]
}

// order the state responses by random numbers
func (c *ConsensusState) orderTxn() {
	// should copy to a new list
	// the original list (c.bgStateResponse) is used for state response look up
	// if modify the original list, GetBGStateResponseById will return a wrong state response
	c.orderedStateResponse = make([]*pb.StateResponseMessage, len(c.bgStateResponse))
	for i, sr := range c.bgStateResponse {
		c.orderedStateResponse[i] = sr
		//logger.Debugf("NONCE cycle %v random %v from server %v", c.cycleId, sr.BgRandom, sr.SenderId)
	}

	// sort by the random number in place
	sort.Slice(c.orderedStateResponse, func(i, j int) bool {
		if c.orderedStateResponse[i].Result.Nonce == c.orderedStateResponse[j].Result.Nonce {
			return c.orderedStateResponse[i].BgId < c.orderedStateResponse[i].BgId
		}
		return c.orderedStateResponse[i].Result.Nonce < c.orderedStateResponse[j].Result.Nonce
	})
}

// return ordered state responses
func (c *ConsensusState) GetOrderedBGStateResponse() []*pb.StateResponseMessage {
	return c.orderedStateResponse
}

// print the ordered state responses for testing
func (c *ConsensusState) printOrderedTxn() {
	n := 0
	for _, sr := range c.orderedStateResponse {
		n += util.CountNumberOfTxnInBftResult(sr.Result)
	}
	logger.Warningf("FINAL cycle %v txn num %v \n", c.cycleId, n)

	// name-removed: memory consumption profiling
	if c.cycleId%100 == 1 {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		totalOccupied := (m.StackSys + m.HeapSys) / 1024 / 1024
		totalInUse := (m.HeapInuse + m.StackInuse) / 1024 / 1024
		logger.Warningf("Current in-use: %v MB, Current consumption: %v MB, %v GCs", totalInUse, totalOccupied, m.NumGC)
		if totalInUse > 15*1024 {
			logger.Fatal("Server terminated due to high memory consumption")
		}
	}

	if n == 0 {
		logger.Error("Unexpected empty cycle")
	}
}
