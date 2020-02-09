package server

import (
	"Polaris/src/common"
	pb "Polaris/src/rpc"
	"math/rand"
	"sync"
)

type CycleStatus int

const (
	// batching txn
	Collecting CycleStatus = iota
	// ready to replicate
	CanReplicate
)

type SkipCycleApplicationMaterial struct {
	skipTo      uint32
	BGApplicant uint32
	proof       string
	signedOn    []byte
	view        int64
	sequence    int64
}

// request manager responses to collect requests from client
// makes a batch sending to Raft layer
// at most one outstanding batch at a time
type RequestManager struct {
	server                       *Server
	incomingTxnChan              chan *pb.TransactionMessage
	readyToReplicateSLRequestMsg chan *pb.SLRequestMessage
	readyToReplicateBodyMsg      chan *pb.TxnBodyMessage
	maxBatchSize                 int

	previousBatchHeader   []*pb.TransactionHeader
	currentBatchHeader    []*pb.TransactionHeader
	currentBatchTxn       []*pb.TransactionMessage
	previousCycleComplete chan bool

	currentCycleStatus  CycleStatus
	receivedSkipCycle   bool
	canSubmitEmptyCycle bool

	skipCycleChan         chan SkipCycleApplicationMaterial
	pendingSkipCycleAlert chan int

	latestSkipCycleApplication SkipCycleApplicationMaterial
	skipCycleMutex             sync.Mutex // Protects latestSkipCycleApplication, receivedSkipCycle

	delayStartBarriers []int
	delayStartChan     chan int
}

func NewRequestManager(
	s *Server,
	timeout int,
	batchSize int32,
	slChan chan *pb.SLRequestMessage,
	txnChan chan *pb.TxnBodyMessage) *RequestManager {

	ret := &RequestManager{
		server:                       s,
		incomingTxnChan:              make(chan *pb.TransactionMessage, 10240),
		skipCycleChan:                make(chan SkipCycleApplicationMaterial, 10240),
		pendingSkipCycleAlert:        make(chan int, 10240),
		previousCycleComplete:        make(chan bool, 10240),
		currentBatchHeader:           make([]*pb.TransactionHeader, 0),
		currentBatchTxn:              make([]*pb.TransactionMessage, 0),
		readyToReplicateSLRequestMsg: slChan,
		readyToReplicateBodyMsg:      txnChan,
		currentCycleStatus:           Collecting,
		receivedSkipCycle:            false,
		canSubmitEmptyCycle:          false,
		maxBatchSize:                 int(batchSize),
		delayStartBarriers:           make([]int, 0),
		delayStartChan:               make(chan int, 10),
	}

	ret.latestSkipCycleApplication.skipTo = 0

	return ret
}

func (r *RequestManager) processSkipCycle() {
	for {
		mat := <-r.skipCycleChan
		logger.Debugf("process skip cycle request (to %v)", mat.skipTo)

		r.skipCycleMutex.Lock()
		if r.receivedSkipCycle && mat.skipTo <= r.latestSkipCycleApplication.skipTo {
			logger.Debugf("cycle %v is already started, last requested cycle is %v", mat.skipTo, r.latestSkipCycleApplication.skipTo)
			// Warning: Need to release the mutex here
			r.skipCycleMutex.Unlock()
			continue
		}

		r.latestSkipCycleApplication = mat

		// Only throw into this channel ONCE
		if !r.receivedSkipCycle {
			r.pendingSkipCycleAlert <- 0
		}

		r.receivedSkipCycle = true
		r.skipCycleMutex.Unlock()
	}
}

func (r *RequestManager) delayStart(cycle int) {
	logger.Warningf("Executing delayStart, waiting for cycle %d", cycle)
	cs := r.server.GetConsensusStateByCycleId(cycle)
	op := NewJustWaitCycleComplete(cs)
	if !cs.AddOPandWait(op) {
		logger.Fatal("AddOPandWait should return true")
	}

	r.delayStartChan <- cycle
}

func (r *RequestManager) tryAppendDelayHelper(cycle int) bool {
	if len(r.delayStartBarriers) > 0 && r.delayStartBarriers[len(r.delayStartBarriers)-1] == cycle {
		// already waiting on this cycle
		return false
	}

	common.AssertTrue(len(r.delayStartBarriers) == 0 || r.delayStartBarriers[len(r.delayStartBarriers)-1] < cycle, "name-removed: The new wait target should be greater than the old ones", logger)

	cs := r.server.GetConsensusStateByCycleId(cycle)
	if cs.IsConsensus() {
		// no need to wait
		return false
	}

	r.delayStartBarriers = append(r.delayStartBarriers, cycle)
	return true
}

func (r *RequestManager) processCurrentCycle() {
	logger.Debugf("Processing current cycle")
	controlDepth := int(r.server.config.GetMaxPipelineDepth()) - r.server.BGsInfo[r.server.BGID].GetSuperLeafNum()
	if controlDepth < 1 {
		controlDepth = 1
	}

	for {
		select {
		case txn := <-r.incomingTxnChan:
			var header []byte
			if r.server.config.IsHeaderBodySeparate() {
				header = txn.TxnHeader
			} else {
				header = txn.TxnContent
			}
			txnHeader := &pb.TransactionHeader{
				SenderId:    txn.SenderId,
				HashContent: header,
			}
			r.currentBatchTxn = append(r.currentBatchTxn, txn)
			r.currentBatchHeader = append(r.currentBatchHeader, txnHeader)
			if r.currentCycleStatus == CanReplicate {
				if r.server.round3LastFinishedCycle >= r.server.round2LastFinishedCycle+1-controlDepth {
					logger.Debugf("previous cycle is already complete, replicate current batch now")
					return
				} else {
					if r.tryAppendDelayHelper(r.server.round2LastFinishedCycle + 1 - controlDepth) {
						go r.delayStart(r.server.round2LastFinishedCycle + 1 - controlDepth)
					}
					continue
				}
			}
		case c := <-r.pendingSkipCycleAlert:
			logger.Debugf("handle pending start cycle request for cycle %v", c)
			// name-removed: discard the alert if it's near
			if r.latestSkipCycleApplication.skipTo <= uint32(r.server.round2LastFinishedCycle)+3 {
				continue
			}

			r.canSubmitEmptyCycle = true
			// when start cycle is received, if the previous cycle is not completed,
			// then continue
			if r.currentCycleStatus == Collecting {
				logger.Debugf("previous cycle is not complete, wait for it complete")
				continue
			} else {
				if r.server.round3LastFinishedCycle >= r.server.round2LastFinishedCycle+1-controlDepth {
					// the previous cycle is completed
					// we can process the current batch txn now
					logger.Debugf("previous cycle is complete, receive start cycle request, start it")
					return
				} else {
					if r.tryAppendDelayHelper(r.server.round2LastFinishedCycle + 1 - controlDepth) {
						go r.delayStart(r.server.round2LastFinishedCycle + 1 - controlDepth)
					}
					continue
				}

			}
		case <-r.previousCycleComplete:
			r.currentCycleStatus = CanReplicate
			if len(r.currentBatchHeader) == 0 && !r.canSubmitEmptyCycle {
				logger.Debugf("previous cycle complete, but no txn is received and no start cycle requst complete, waiting for txn or start cycle request")
				continue
			}

			if r.server.round3LastFinishedCycle >= r.server.round2LastFinishedCycle+1-controlDepth {
				// the previous cycle is completed
				// we can process the current batch txn now
				logger.Debugf("previous cycle complete, and some txn is received, replicate, txn num %v ", len(r.currentBatchHeader))
				return
			} else {
				if r.tryAppendDelayHelper(r.server.round2LastFinishedCycle + 1 - controlDepth) {
					go r.delayStart(r.server.round2LastFinishedCycle + 1 - controlDepth)
				}
				continue
			}

		case cycle := <-r.delayStartChan:
			idxMatch := -1
			for idx := 0; idx < len(r.delayStartBarriers); idx++ {
				if r.delayStartBarriers[idx] == cycle {
					idxMatch = idx
					break
				}
			}

			if idxMatch == -1 {
				logger.Errorf("Waiting for cycles %v but got notification of cycle %d", r.delayStartBarriers, cycle)
				continue
			}

			r.delayStartBarriers = append(r.delayStartBarriers[0:idxMatch], r.delayStartBarriers[idxMatch+1:]...)

			common.AssertTrue(r.currentCycleStatus == CanReplicate, "Delay Start should only be triggered when the previous cycle has completed", logger)
			common.AssertTrue(len(r.currentBatchHeader) != 0 || r.canSubmitEmptyCycle, "Cycle should not be empty if there is no SkipCycle request", logger)

			if len(r.delayStartBarriers) == 0 {
				return
			} else {
				continue
			}
		}
	}
}

func (r *RequestManager) collectBatch() {
	r.skipCycleMutex.Lock()
	defer r.skipCycleMutex.Unlock()

	controlDepth := int(r.server.config.GetMaxPipelineDepth()) - r.server.BGsInfo[r.server.BGID].GetSuperLeafNum()
	if controlDepth < 1 {
		controlDepth = 1
	}

	bsize := len(r.currentBatchHeader)
	logger.Debugf("current pending txn: %v, max batchsize %v", bsize, r.maxBatchSize)
	if bsize > r.maxBatchSize {
		bsize = r.maxBatchSize
	}

	txnHeaderList := make([]*pb.TransactionHeader, bsize)
	txnMsgList := make([]*pb.TransactionMessage, bsize)
	r.previousBatchHeader = make([]*pb.TransactionHeader, bsize)
	for i := 0; i < bsize; i++ {
		t := r.currentBatchHeader[i]
		txnHeaderList[i] = t
		r.previousBatchHeader[i] = t
	}

	for i := 0; i < bsize; i++ {
		txnMsgList[i] = r.currentBatchTxn[i]
	}

	txnHeaderBlock := &pb.TxnHeaderBlock{
		TxnHeaderList: txnHeaderList,
	}

	slRequest := &pb.SLRequestMessage{
		SenderId:       r.server.ID,
		Nonce:          rand.Int63(),
		TxnHeaderBlock: txnHeaderBlock,
	}

	if slRequest.TxnHeaderBlock == nil || slRequest.TxnHeaderBlock.TxnHeaderList == nil {
		logger.Fatal("Txn cannot be nil")
	}

	if r.receivedSkipCycle {
		// SLRequestMessage should carry SkipCycle information
		slRequest.CycleSuggestion = r.latestSkipCycleApplication.skipTo
		// name-removed: Should not violate the delay start rules
		if slRequest.CycleSuggestion > uint32(r.server.round3LastFinishedCycle+controlDepth) {
			slRequest.CycleSuggestion = uint32(r.server.round3LastFinishedCycle + controlDepth)
		}

		slRequest.WhichGroup = r.latestSkipCycleApplication.BGApplicant
		slRequest.Proof = r.latestSkipCycleApplication.proof
		slRequest.Hash = r.latestSkipCycleApplication.signedOn
		slRequest.ForeignView = r.latestSkipCycleApplication.view
		slRequest.ForeignSequence = r.latestSkipCycleApplication.sequence
		logger.Warningf("This batch has a SkipCycle attachment. Requested cycle ID = %d, from BG %d.", r.latestSkipCycleApplication.skipTo, r.latestSkipCycleApplication.BGApplicant)
	}

	txnMsgBlock := &pb.TxnMessageBlock{
		TxnMessageList: txnMsgList,
	}

	txnBody := &pb.TxnBodyMessage{
		SenderId:        r.server.ID,
		SuperLeafId:     int32(r.server.SuperLeafID),
		RaftId:          int32(rand.Intn(len(r.server.raftPeers)) + 1),
		TxnMessageBlock: txnMsgBlock,
	}

	logger.Debugf("txn block is ready to replicate")
	r.readyToReplicateSLRequestMsg <- slRequest
	if r.server.config.IsHeaderBodySeparate() {
		r.readyToReplicateBodyMsg <- txnBody
	}

	if bsize == len(r.currentBatchHeader) {
		r.currentBatchHeader = make([]*pb.TransactionHeader, 0)
		r.currentBatchTxn = make([]*pb.TransactionMessage, 0)
	} else {
		// carry on to the next batch
		r.currentBatchHeader = r.currentBatchHeader[bsize:]
		r.currentBatchTxn = r.currentBatchTxn[bsize:]
	}

	// Reset SkipCycle flag variables
	r.canSubmitEmptyCycle = false
	r.receivedSkipCycle = false
	r.latestSkipCycleApplication.skipTo = 0
}

func (r *RequestManager) Start() {
	logger.Debugf("request manager starts ...")
	go r.processSkipCycle()
	// initially, the previous cycle is completed
	r.previousCycleComplete <- true
	for {
		r.processCurrentCycle()
		r.collectBatch()
		// reset status
		r.currentCycleStatus = Collecting
		r.delayStartBarriers = make([]int, 0)
	}
}

func (r *RequestManager) AddSkipCycle(req SkipCycleApplicationMaterial) {
	logger.Warningf("add skip cycle request for cycle %v", req.skipTo)
	r.skipCycleChan <- req
}

func (r *RequestManager) GetPreviousBatch() []*pb.TransactionHeader {
	return r.previousBatchHeader
}
