package server

import (
	pb "Polaris/src/rpc"
	"bytes"
	"encoding/gob"

	"time"

	"github.com/coreos/etcd/snap"
)

const SLRequestMsg = "R"
const StateResponseMsg = "S"
const RepresentitiveMsg = "T"
const TxnBodyMsg = "B"

type representitive struct {
	ServerList []string
	CycleId    int
}

type Raft struct {
	//rawTxnBlock chan transaction.Block

	slRequestChan chan *pb.SLRequestMessage

	txnBodyChan chan *pb.TxnBodyMessage

	stateResponseChan chan *pb.StateResponseMessage

	//representitiveChan chan *representitive

	server *Server
	// propose the message
	proposeC    chan<- string
	snapshotter *snap.Snapshotter
	//	cycleId     int
}

func NewRaft(server *Server,
	snapshotter *snap.Snapshotter,
	proposeC chan<- string,
	commitC <-chan *string,
	errorC <-chan error) *Raft {
	raft := &Raft{
		slRequestChan:     make(chan *pb.SLRequestMessage, CHANNELLEN),
		txnBodyChan:       make(chan *pb.TxnBodyMessage, CHANNELLEN),
		stateResponseChan: make(chan *pb.StateResponseMessage, CHANNELLEN),
		//representitiveChan: make(chan *representitive, CHANNELLEN),
		server:      server,
		proposeC:    proposeC,
		snapshotter: snapshotter,
	}
	go raft.applyChanges(commitC, errorC)
	return raft
}

func (r *Raft) GetSnapshot() ([]byte, error) {
	return make([]byte, 0), nil
}

func (r *Raft) processMessage() {
	counter := 1
	for {
		logger.Debugf("Process txn block raft...")
		select {
		case sl := <-r.slRequestChan:
			r.applySLRequest(sl)
			logger.Debugf("Server %v receive txHeaderblock %v", r.server.ID, counter)
			counter++
		case stateResponse := <-r.stateResponseChan:
			logger.Debugf("Sever %v receive state response from server %v bg %v cycle %v",
				r.server.ID, stateResponse.SenderId, stateResponse.BgId, stateResponse.CycleId)
			r.replicateStateResponse(stateResponse)
		case tb := <-r.txnBodyChan:
			r.replicateTxnBody(tb)
			//logger.Debugf("replicate txn body %v", tb)
		}
	}
}

func (r *Raft) AddStateResponse(stateResponse *pb.StateResponseMessage) {
	r.stateResponseChan <- stateResponse
}

// single thread process all the requests
func (r *Raft) Start() {
	logger.Debugf("Server %v:%v start raft", r.server.Address, r.server.RPCPort)
	r.processMessage()
}

// replicate a batch of transactions
func (r *Raft) replicateSLRequest(sl *pb.SLRequestMessage) {

	logger.Debugf("Leader %v replicate block txn num %v", r.server.ID, len(sl.TxnHeaderBlock.TxnHeaderList))

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(sl); err != nil {
		logger.Errorf("request encoding error: %v", err)
	}

	r.proposeC <- SLRequestMsg + string(buf.Bytes())
}

func (r *Raft) replicateTxnBody(txnBody *pb.TxnBodyMessage) {

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(txnBody); err != nil {
		logger.Errorf("request encoding error: %v", err)
	}

	r.proposeC <- TxnBodyMsg + string(buf.Bytes())

}

// replicate state response
func (r *Raft) replicateStateResponse(sp *pb.StateResponseMessage) {
	// check is the state response is already added
	// if yes, no nothing
	if r.server.CheckBGState(int(sp.CycleId), int(sp.BgId)) {
		logger.Debugf("bg state for cycle %v bg %v is already in the log", sp.CycleId, sp.BgId)
		return
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(sp); err != nil {
		logger.Errorf("request encoding error: %v", err)
	}
	logger.Debugf("replicate state response for cycle %v bg %v", sp.CycleId, sp.BgId)
	r.proposeC <- StateResponseMsg + string(buf.Bytes())
}

// replicate state representitives
func (r *Raft) replicateRepresentitive(rp *representitive) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rp); err != nil {
		logger.Errorf("request encoding error: %v", err)
	}
	logger.Debugf("replicate representitive for cycle %v", rp.CycleId)
	r.proposeC <- RepresentitiveMsg + string(buf.Bytes())
}

func (r *Raft) applyChanges(commitC <-chan *string, errorC <-chan error) {

	for {
		select {
		case data := <-commitC:
			if data == nil {
				logger.Warning("raft return nil data")
				continue
			}

			prefix := string((*data)[0])
			msg := string((*data)[1:])

			decoder := gob.NewDecoder(bytes.NewBufferString(msg))

			switch prefix {
			case SLRequestMsg:
				info := new(pb.SLRequestMessage)
				if err := decoder.Decode(info); err != nil {
					logger.Errorf("Decoding Error %v", err)
				}
				r.applySLRequest(info)
			case TxnBodyMsg:
				info := new(pb.TxnBodyMessage)
				if err := decoder.Decode(info); err != nil {
					logger.Errorf("Decoding Error %v", err)
				}
				r.applyTxnBodyMsg(info)
			case StateResponseMsg:
				info := new(pb.StateResponseMessage)
				if err := decoder.Decode(info); err != nil {
					logger.Errorf("Decoding Error %v", err)
				}
				r.applyStateResponseMsg(info)
			case RepresentitiveMsg:
				info := new(representitive)
				if err := decoder.Decode(info); err != nil {
					logger.Errorf("Decoding Error %v", err)
				}
				r.applyRepresentitiveMsg(info)
			}
		case e := <-errorC:
			logger.Error(e)
		}
	}
}

// the monitor selects the representitives
func (r *Raft) SelectRepresentitives(cycleId int) {
	logger.Debugf("select representitive for cycle %v", cycleId)
	if !r.server.IsLeader() {
		logger.Fatal("to select representitive, the server should be leader")
	}
	// RaftID starts at 1
	//id := r.server.RaftID % len(r.server.rpcPeers)

	//serverId := r.server.rpcPeers[id]
	serverList := make([]string, 0)
	serverList = append(serverList, r.server.GetSelfBGInfo().GetRandomNode(r.server.SuperLeafID))
	//serverList = append(serverList, serverId)
	rep := &representitive{
		ServerList: serverList,
		CycleId:    cycleId,
	}

	r.replicateRepresentitive(rep)
}

func (r *Raft) applySLRequest(req *pb.SLRequestMessage) {
	if req.SenderId == r.server.ID {
		logger.Debugf("leader %v replicated txnblock %v", req.SenderId, len(req.TxnHeaderBlock.TxnHeaderList))
		//logger.Debugf("Send txn num %v to sbft, content %v", len(req.TxnHeaderBlock.TxnHeaderList), req)
		logger.Warningf("Send txn num %v to sbft", len(req.TxnHeaderBlock.TxnHeaderList))

		conn := r.server.GetSBFTClientConnection()

		conn.AddEvent(
			NewOrderRequestEvent(req, 2*time.Second, r.server.SBFTClientAddressForBFTLayer),
		)
	} else {
		logger.Debugf("follower %v replicated txnblock %v", req.SenderId, req.TxnHeaderBlock.TxnHeaderList)
	}
}

func (r *Raft) applyTxnBodyMsg(req *pb.TxnBodyMessage) {
	go r.server.txnStore.PutTxnMsgBlock(req)
	if int(req.SuperLeafId) != r.server.SuperLeafID {
		return
	}
	//if req.SenderId == r.server.ID {
	if int(req.RaftId) == r.server.RaftID {
		// send to other super-leaf leaders
		n := r.server.GetSelfBGInfo().GetSuperLeafNum()
		for i := 0; i < n; i++ {
			// skip own super-leaf
			if i == r.server.SuperLeafID {
				continue
			}
			// randomly pick a memeber in super-leaf i
			// if it is not the leader it will forward to leader
			//dstAddr := r.server.GetSelfBGInfo().GetLeader(i)
			dstAddr := r.server.GetSelfBGInfo().GetRandomNode(i)
			logger.Warningf("send the txn num %v to server %v superleaf %v", len(req.TxnMessageBlock.TxnMessageList), dstAddr, i)
			conn := r.server.GetSelfBGInfo().GetConnection(dstAddr)
			conn.AddEvent(NewAcceptTxnBodyEvent(req, 10*time.Second, dstAddr, r.server))
		}
	}
}

func (r *Raft) applyStateResponseMsg(req *pb.StateResponseMessage) {
	// if the state is not added, then add
	// otherwise, ignore
	logger.Debugf("finish replicated state response for cycle %v bg %v from server %v", req.CycleId, req.BgId, req.SenderId)
	logger.Debugf("Server %v add state response for cycle %v bg %v", r.server.ID, req.CycleId, req.BgId)

	r.server.AddBGState(req)
}

func (r *Raft) applyRepresentitiveMsg(req *representitive) {
	for _, s := range req.ServerList {
		logger.Debugf("Server %v is representitives for cycle %v", req.ServerList, req.CycleId)
		if r.server.GetServerAddr() == s {
			logger.Debugf("Server %v is representitive for cycle %v", r.server.ID, req.CycleId)
			r.server.disseminationLayer.AddStateRequestForCycle(req.CycleId)
		}
	}

}

func (r *Raft) HandleOwnStateResponse(stateResponse *pb.StateResponseMessage) {
	// use Raft to replicate the stateReponse
	logger.Debugf("handle own state response for cycle %v", stateResponse.CycleId)
	r.AddStateResponse(stateResponse)

	// select representitive using Raft for cycle cycleId
	// once the server selected as a representitive,
	// it will push a cycleId into stateRequestChan
	if len(r.server.BGsInfo) > 1 {
		r.SelectRepresentitives(int(stateResponse.CycleId))
	}
}
