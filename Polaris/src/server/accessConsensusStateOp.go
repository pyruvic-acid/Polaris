package server

import (
	pb "Polaris/src/rpc"
)

type accessConsensusStateOp interface {
	Execute()
	BlockOwner() bool
}

type AddStateResponseOp struct {
	stateResponse  *pb.StateResponseMessage
	consensusState *ConsensusState
}

func NewAddStateResponseOp(
	stateResponse *pb.StateResponseMessage,
	cs *ConsensusState) *AddStateResponseOp {
	return &AddStateResponseOp{stateResponse, cs}
}

func (o *AddStateResponseOp) Execute() {
	logger.Debugf("Execute Add state response from server %v bg %v cycle %v",
		o.stateResponse.SenderId, o.stateResponse.BgId, o.stateResponse.CycleId)
	o.consensusState.AddBGState(o.stateResponse)
}

func (o *AddStateResponseOp) BlockOwner() bool {
	// should not block
	return false
}

type CheckBGStateOp struct {
	bgId           int
	result         bool
	consensusState *ConsensusState
	wait           chan bool
}

func NewCheckBGStateOp(bgId int, consensusState *ConsensusState) *CheckBGStateOp {
	logger.Debugf("create new check bg state op for bg %v cycle %v",
		bgId, consensusState.cycleId)
	o := &CheckBGStateOp{
		bgId:           bgId,
		result:         false,
		consensusState: consensusState,
		wait:           make(chan bool),
	}
	return o
}

func (o *CheckBGStateOp) Execute() {
	logger.Debugf("Execute check bg state for BG %v cycleId %v",
		o.bgId, o.consensusState.cycleId)
	o.result = o.consensusState.IsBGStateExist(o.bgId)
	o.wait <- true
}

func (o *CheckBGStateOp) BlockOwner() bool {
	return <-o.wait
}

func (o *CheckBGStateOp) GetResult() bool {
	return o.result
}

type CheckCycleCompleteOp struct {
	result         bool
	wait           chan bool
	consensusState *ConsensusState
}

func NewCheckCycleCompleteOp(consensusState *ConsensusState) *CheckCycleCompleteOp {
	o := &CheckCycleCompleteOp{
		result:         false,
		wait:           make(chan bool),
		consensusState: consensusState,
	}

	return o
}

func (o *CheckCycleCompleteOp) Execute() {
	logger.Debugf("Execute check cycle complete for cycle %v",
		o.consensusState.cycleId)
	o.result = o.consensusState.IsConsensus()
	o.wait <- true
}

func (o *CheckCycleCompleteOp) BlockOwner() bool {
	return <-o.wait
}

func (o *CheckCycleCompleteOp) GetResult() bool {
	return o.result
}

type GetAndWaitOwnBGStateResponseOp struct {
	bgId           int
	wait           chan bool
	result         *pb.StateResponseMessage
	consensusState *ConsensusState
}

func NewGetAndWaitOwnBGStateResponseOp(bgId int,
	consensusState *ConsensusState) *GetAndWaitOwnBGStateResponseOp {
	o := &GetAndWaitOwnBGStateResponseOp{
		bgId:           bgId,
		wait:           make(chan bool),
		result:         nil,
		consensusState: consensusState,
	}
	return o
}

func (o *GetAndWaitOwnBGStateResponseOp) Execute() {
	logger.Debugf("Execute get own bg %v state response for cycle %v",
		o.bgId, o.consensusState.cycleId)

	o.result = o.consensusState.GetBGStateResponseById(o.bgId)
	if o.result == nil {
		o.consensusState.AddWaitOwnStateResponse(o)
	} else {
		o.wait <- true
	}
}

func (o *GetAndWaitOwnBGStateResponseOp) BlockOwner() bool {
	return <-o.wait
}

func (o *GetAndWaitOwnBGStateResponseOp) GetResult() *pb.StateResponseMessage {
	return o.result
}

type GetBGStateResponseOp struct {
	bgId           int
	wait           chan bool
	result         *pb.StateResponseMessage
	consensusState *ConsensusState
}

func NewGetBGStateResponseOp(bgId int,
	consensusState *ConsensusState) *GetBGStateResponseOp {

	o := &GetBGStateResponseOp{
		bgId:           bgId,
		wait:           make(chan bool),
		result:         nil,
		consensusState: consensusState,
	}

	return o
}

func (o *GetBGStateResponseOp) Execute() {
	logger.Debugf("Execute get bg %v state response for cycle %v", o.bgId, o.consensusState.cycleId)
	o.result = o.consensusState.GetBGStateResponseById(o.bgId)
	o.wait <- true
}

func (o *GetBGStateResponseOp) BlockOwner() bool {
	return <-o.wait
}

func (o *GetBGStateResponseOp) GetResult() *pb.StateResponseMessage {
	return o.result
}

type WaitCycleComplete struct {
	consensusState *ConsensusState
	result         []*pb.StateResponseMessage
	wait           chan bool
}

func NewWaitCycleComplete(cs *ConsensusState) *WaitCycleComplete {
	return &WaitCycleComplete{cs, make([]*pb.StateResponseMessage, 0), make(chan bool)}
}

func (o *WaitCycleComplete) Execute() {
	if o.consensusState.IsConsensus() {
		o.result = o.consensusState.GetOrderedBGStateResponse()
		o.wait <- true
		return
	}
	o.consensusState.AddWaitCompleteOp(o)
}

func (o *WaitCycleComplete) BlockOwner() bool {
	return <-o.wait
}

func (o *WaitCycleComplete) GetResult() []*pb.StateResponseMessage {
	return o.result
}

type JustWaitCycleComplete struct {
	consensusState *ConsensusState
	wait           chan bool
}

func NewJustWaitCycleComplete(cs *ConsensusState) *JustWaitCycleComplete {
	return &JustWaitCycleComplete{cs, make(chan bool)}
}

func (o *JustWaitCycleComplete) Execute() {
	if o.consensusState.IsConsensus() {
		o.wait <- true
		return
	}
	o.consensusState.AddJustWaitCompleteOp(o)
}

func (o *JustWaitCycleComplete) BlockOwner() bool {
	return <-o.wait
}

type GetOrWaitBGStateResponse struct {
	consensusState *ConsensusState
	stateRequest   *pb.StateRequestMessage
	result         *pb.StateResponseMessage
	requestBGID    int
	wait           chan bool
}

func NewGetOrWaitBGStateResponseOp(stateRequest *pb.StateRequestMessage, cs *ConsensusState, requestBGID int) *GetOrWaitBGStateResponse {
	o := &GetOrWaitBGStateResponse{
		consensusState: cs,
		stateRequest:   stateRequest,
		result:         nil,
		requestBGID:    requestBGID,
		wait:           make(chan bool),
	}
	return o
}

func (o *GetOrWaitBGStateResponse) Execute() {
	logger.Debugf("Execute get or wait bg state response op for server %v cycle %v bg %v",
		o.stateRequest.SenderId, o.stateRequest.CycleId, o.requestBGID)

	if o.consensusState.IsBGStateExist(o.requestBGID) {
		logger.Debugf("State response for bg %v cycle %v is ready for server %v",
			o.requestBGID, o.stateRequest.CycleId, o.stateRequest.SenderId)

		o.result = o.consensusState.GetBGStateResponseById(o.requestBGID)
	} else {
		logger.Debugf("State response for bg %v cycle %v is not ready for server %v add to waiting list",
			o.requestBGID, o.stateRequest.CycleId, o.stateRequest.SenderId)

		o.consensusState.AddPendingStateRequestOp(o)
	}
	o.wait <- true
}

func (o *GetOrWaitBGStateResponse) BlockOwner() bool {
	return <-o.wait
}

func (o *GetOrWaitBGStateResponse) GetResult() *pb.StateResponseMessage {
	return o.result
}
