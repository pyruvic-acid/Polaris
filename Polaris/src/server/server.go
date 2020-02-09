package server

import (
	"Polaris/src/common"
	pb "Polaris/src/rpc"
	"Polaris/src/util"

	"net"

	"Polaris/src/byzantineGroup"
	"Polaris/src/configuration"
	"Polaris/src/raftnode"

	"github.com/op/go-logging"
	grpcpool "github.com/processout/grpc-go-pool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"encoding/binary"
	"encoding/json"
	"io/ioutil"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"

	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var logger = logging.MustGetLogger("server")
var RaftInputChannelSize int = 10240
var RaftOutputChannelSize int = 10240

const CHANNELLEN = 10240

type Server struct {
	// server info
	ID         string
	Address    string
	RPCPort    string
	rpcPeers   []string
	grpcServer *grpc.Server
	addr       string // Address:RPCPort

	RaftID    int
	RaftPort  string
	raftPeers []string
	join      bool
	raftNode  *raftnode.RaftNode

	// client request manager
	requestManager *RequestManager
	//raft layer
	raftLayer *Raft
	// dissemination layer
	disseminationLayer *Dissemination

	// current pbft leader Id
	currentBGLeaderSuperLeadId int

	BGsInfo     []*byzantineGroup.ByzantineGroup
	SuperLeafID int
	BGID        int

	getLeaderID func() uint64

	config *configuration.Configuration

	// cycleId -> consensusState
	log           []*ConsensusState
	logLock       sync.RWMutex
	skipCycleHash [][]byte

	SBFTClientAddressForMembership string
	SBFTClientAddressForBFTLayer   string
	SBFTReplicaAddressForBFTResult string
	SBFTResultVerifierAddress      string

	SBFTClientConnection *byzantineGroup.Connection

	resultReceiver *BftResultReceiver
	resultVerifier *BftResultVerifier

	txnStore *TxnBodyStore

	tcpManager       *TCPManager
	serverTCPAddrMap map[string]string

	round2LastFinishedCycle int
	round3LastFinishedCycle int
}

func NewServer(configFilePath string, configHome string) *Server {
	logger.Debug(configFilePath)
	raw, err := ioutil.ReadFile(configFilePath)
	common.AssertNil(err, "Cannot read config file", logger)
	c := make(map[string]interface{})
	err = json.Unmarshal(raw, &c)
	common.AssertNil(err, "JSON parsing error", logger)

	server := &Server{
		ID:      c["id"].(string),
		Address: c["address"].(string),
		RPCPort: c["rpc_port"].(string),

		RaftID:                         int(c["raft_id"].(float64)),
		RaftPort:                       c["raft_port"].(string),
		join:                           c["join"].(bool),
		BGID:                           int(c["bg_id"].(float64)),
		SBFTClientAddressForMembership: c["SBFT_client_membership"].(string),
		SBFTClientAddressForBFTLayer:   c["SBFT_client_bft_layer"].(string),
		SBFTReplicaAddressForBFTResult: c["SBFT_replica_bft_result"].(string),
		SBFTResultVerifierAddress:      c["SBFT_result_verifier"].(string),
		disseminationLayer:             NewDissemination(),
		currentBGLeaderSuperLeadId:     0,
		txnStore:                       NewTxnBodyStore(),
		serverTCPAddrMap:               make(map[string]string),
		skipCycleHash:                  make([][]byte, 0, 512),
		round2LastFinishedCycle:        0,
		round3LastFinishedCycle:        0,
		log:                            make([]*ConsensusState, common.MaxCycleNumberInStorage),
	}

	for i := 0; i < common.MaxCycleNumberInStorage; i++ {
		// Fill it with -1
		server.log[i] = NewConsensusState(len(server.BGsInfo), -1, server)
	}

	i1, err := strconv.Atoi(server.RPCPort)
	common.AssertNil(err, "RPC port cannot be recognized.", logger)
	tcpPort := common.CalcTCPPortFromRPCPort(i1)

	server.tcpManager = NewTCPManager(strconv.Itoa(tcpPort))
	server.tcpManager.SetServer(server)

	logger.Infof("Server %s start ", server.Address+":"+server.RPCPort)

	for _, s := range c["raft_peers"].([]interface{}) {
		server.raftPeers = append(server.raftPeers, s.(string))
	}
	for _, s := range c["rpc_peers"].([]interface{}) {
		server.rpcPeers = append(server.rpcPeers, s.(string))
	}

	server.resultReceiver = NewBftResultReceiver(server.SBFTReplicaAddressForBFTResult)
	server.resultVerifier = NewBftResultVerifier(server.SBFTResultVerifierAddress)

	server.disseminationLayer.SetServer(server)

	server.config = configuration.NewConfiguration(server.ID, configHome)

	server.addr = server.Address + ":" + server.RPCPort
	// initialize byzantine groups (list of superleafs)
	for _, bg := range c["byzantine_groups"].([]interface{}) {
		bgInfo := byzantineGroup.NewByzantineGroup(bg)
		server.BGsInfo = append(server.BGsInfo, bgInfo)
	}

	var succ bool
	server.SuperLeafID, succ = server.BGsInfo[server.BGID].GetSuperLeafIdByServer(
		server.Address + ":" + server.RPCPort)
	common.AssertTrue(succ, "Cannot fetch SuperleafID", logger)
	logger.Infof("Server %s SuperLeafId %d", server.Address+":"+server.RPCPort, server.SuperLeafID)

	//create grpc server
	sopts := []grpc.ServerOption{grpc.InitialWindowSize(2000000)}
	sopts = append(sopts, grpc.InitialConnWindowSize(2000000))
	server.grpcServer = grpc.NewServer(sopts...)
	pb.RegisterPolarisAllServer(server.grpcServer, server)
	reflection.Register(server.grpcServer)
	logger.Info("GRPC server created")

	return server
}

func (s *Server) Start() {

	logger.Debugf("Server %v start...", s.ID)

	// initialize Raft layer
	proposeC := make(chan string)
	defer close(proposeC)
	confChangeC := make(chan raftpb.ConfChange)
	defer close(confChangeC)

	var raft *Raft
	getSnapshot := func() ([]byte, error) { return raft.GetSnapshot() }
	commitC, errorC, snapshotterReady, getLeaderID, raftNode, leaderC := raftnode.NewRaftNode(
		s.RaftID, s.RaftPort, s.raftPeers, s.join, getSnapshot, proposeC, confChangeC, RaftOutputChannelSize)
	s.getLeaderID = getLeaderID
	s.raftNode = raftNode

	s.raftLayer = NewRaft(s, <-snapshotterReady, proposeC, commitC, errorC)

	s.requestManager = NewRequestManager(s,
		int(s.config.GetClientRequestCycleTimeout()),
		s.config.GetMaxBatchSize(),
		s.raftLayer.slRequestChan,
		s.raftLayer.txnBodyChan)

	// wait for raft leader election
	<-leaderC

	go s.requestManager.Start()
	go s.raftLayer.Start()

	go s.disseminationLayer.Start()

	if s.IsLeader() {
		// create a thread receiving the result from SBFT replica
		go s.resultReceiver.Start()
		logger.Debugf("leader %v start process sbft result thread", s.ID)
		// create a thread handling the result from SBFT replica
		go s.processSBFTResult()
	}

	s.saveLeaderAddress()

	s.tcpManager.Start()
	tcpAddrList := make([]string, 0)
	for _, bg := range s.BGsInfo {
		for _, sAddr := range bg.GetServerList() {
			items := strings.Split(sAddr, ":")
			portNum, _ := strconv.Atoi(items[1])
			tPort := common.CalcTCPPortFromRPCPort(portNum)
			tAddr := items[0] + ":" + strconv.Itoa(tPort)
			s.serverTCPAddrMap[sAddr] = tAddr
			tcpAddrList = append(tcpAddrList, tAddr)
		}
	}
	logger.Debugf("tpc server addrs %v", tcpAddrList)
	s.tcpManager.DialAll(tcpAddrList)

	// Start a RPC Listener
	RPCListener, err := net.Listen("tcp", ":"+s.RPCPort)
	if err != nil {
		logger.Errorf("Failed to listen port %s, %v", s.RPCPort, err)
	}
	err = s.grpcServer.Serve(RPCListener)
	if err != nil {
		logger.Errorf("Cannot start to serve RPC calls %v", err)
	}

}

// get monitor id (the leader of superleaf(Raft group))
func (s *Server) GetLeader() string {

	id := s.getLeaderID()
	if id == 0 {
		return ""
	}
	leader := s.rpcPeers[id-1]
	go s.BGsInfo[s.BGID].UpdateLeader(leader)
	logger.Debugf("server %v get leader %v, raft id %v", s.addr, leader, id)
	return leader
}

func (s *Server) IsLeader() bool {
	// when the raft leader call getLeaderID, it will return 0
	// it only shows in the first time
	return int(s.getLeaderID()) == s.RaftID || int(s.getLeaderID()) == 0
}

func (s *Server) IsBGLeader() bool {
	if s.currentBGLeaderSuperLeadId != s.SuperLeafID {
		return false
	}

	return s.IsLeader()
}

func (s *Server) saveLeaderAddress() {
	fName := fmt.Sprintf("./leader_%v.log", s.ID)
	f, err := os.Create(fName)
	if err != nil {
		logger.Fatalf("create leader log file error %v", err)
	}

	defer f.Close()

	leader := s.GetLeader()
	if leader == "" {
		leader = s.addr
	}
	logger.Debugf("save leader id %v", leader)
	f.WriteString(leader)
	f.Sync()
}

func (s *Server) GetRaftTerm() uint64 {
	if s.raftNode == nil {
		logger.Error("Raft Node is nil!")
		return raft.InvalidRaftTerm
	}
	return s.raftNode.GetRaftTerm()
}

func (s *Server) GetSBFTClientConnection() *byzantineGroup.Connection {
	if s.SBFTClientConnection == nil {
		s.SBFTClientConnection = byzantineGroup.NewConnection(s.SBFTClientAddressForBFTLayer, queueLen, 1)
	}

	return s.SBFTClientConnection
}

func (s *Server) handleRPCError(err error) (*grpcpool.Pool, bool) {

	errCode := grpc.Code(err)
	if errCode == common.NotLeaderErrorCode {
		leaderId := grpc.ErrorDesc(err)
		s.BGsInfo[s.BGID].UpdateLeader(leaderId)
		conn := s.BGsInfo[s.BGID].GetConnection(leaderId).GetConnectionPool()
		return conn, true
	}
	logger.Warningf("Unhandled ERROR: %v", err)
	return nil, false
}

// list of monitors (leaders of superleafs)
func (s *Server) GetBGMembers() []string {
	leaders := make([]string, 0)
	for i := 0; i < s.BGsInfo[s.BGID].GetSuperLeafNum(); i++ {
		l := s.BGsInfo[s.BGID].GetLeader(i)
		if l == s.GetServerAddr() {
			continue
		}
		leaders = append(leaders, l)
	}
	return leaders
}

func (s *Server) GetServerAddr() string {
	return s.addr
}

// return consensusState for cycleId
func (s *Server) GetConsensusStateByCycleId(cycleId int) *ConsensusState {
	s.logLock.Lock()
	defer s.logLock.Unlock()

	common.AssertTrue(cycleId >= 0, "cycleID cannot be negative!", logger)
	common.AssertTrue(cycleId > s.round2LastFinishedCycle-common.MaxCycleNumberInStorage, "Cannot access stale cycles!", logger)

	// cycle may not complete in order

	index := cycleId % common.MaxCycleNumberInStorage
	if s.log[index].cycleId != cycleId {
		// name-removed: Here we overwrite stale records
		s.log[index] = NewConsensusState(len(s.BGsInfo), cycleId, s)
	}

	return s.log[index]
}

// check if the server recieves the state response of bgId for cycleId
// return true: already received
//        false: not received
func (s *Server) CheckBGState(cycleId int, bgId int) bool {
	logger.Debugf("check bg state response for cycle %v bgId %v", cycleId, bgId)
	cs := s.GetConsensusStateByCycleId(cycleId)

	op := NewCheckBGStateOp(bgId, cs)
	if !cs.AddOPandWait(op) {
		logger.Fatal("Error: should return true")
	}
	logger.Debugf("state response for cycle %v bgId %v is %v",
		cycleId, bgId, op.GetResult())
	return op.GetResult()
}

func (s *Server) CheckStateRequest(sp *pb.StateRequestMessage) bool {
	return s.checkRequestQC(sp)
}

func (s *Server) CheckStateResponse(sp *pb.StateResponseMessage) bool {
	r := s.checkQC(sp)

	if r {
		// if QC check ok, then replicate to the Raft group
		s.disseminationLayer.AddStateResponse(sp)
	}

	return r
}

// if the state response is ready, then get the state response
// otherwise return nil and put request to pending list
func (s *Server) GetOrWaitBGStateResponse(stateRequest *pb.StateRequestMessage) *pb.StateResponseMessage {
	logger.Debugf("Get state response for cycle %v from server %v",
		stateRequest.CycleId, stateRequest.SenderId)
	cs := s.GetConsensusStateByCycleId(int(stateRequest.CycleId))

	op := NewGetOrWaitBGStateResponseOp(stateRequest, cs, s.BGID)

	if !cs.AddOPandWait(op) {
		logger.Fatal("Error: should return true")
	}
	logger.Debugf("get or wait state response for cycle %v bgId %v is %v for server %v",
		stateRequest.CycleId, s.BGID, op.GetResult(), stateRequest.SenderId)
	return op.GetResult()
}

// get the own bg info
func (s *Server) GetSelfBGInfo() *byzantineGroup.ByzantineGroup {
	return s.BGsInfo[s.BGID]
}

// add the state response into log
func (s *Server) AddBGState(sp *pb.StateResponseMessage) {
	logger.Debugf("Add state response for bg %v cycleId %v", sp.BgId, sp.CycleId)
	cs := s.GetConsensusStateByCycleId(int(sp.CycleId))

	op := NewAddStateResponseOp(sp, cs)

	cs.AddOPandWait(op)

	for cycleID := s.round3LastFinishedCycle + 1; ; cycleID++ {
		if s.CheckCycleComplete(cycleID) {
			s.round3LastFinishedCycle = cycleID

			// name-removed: discard Raft logs
			if cycleID%100 == 0 {
				stat := s.raftNode.Node.Status()
				index := stat.Applied
				s.raftNode.RaftStorage.Compact(index)
			}

		} else {
			break
		}
	}

	logger.Debugf("Successfully add state response for bg %v cycle %v", sp.BgId, sp.CycleId)
}

// check if the cycle completes: the server received all the state response of BG for cycleId
// return true: received all state response
//        false: at least one state response is missing
func (s *Server) CheckCycleComplete(cycleId int) bool {
	logger.Debugf("Check cycle complete cycleId %v", cycleId)
	cs := s.GetConsensusStateByCycleId(cycleId)

	op := NewCheckCycleCompleteOp(cs)
	if !cs.AddOPandWait(op) {
		logger.Fatal("Should return true check cycle complete")
	}

	return op.GetResult()
}

func (s *Server) GetAndWaitOwnBGStateResponse(cycleId int) *pb.StateResponseMessage {
	logger.Debugf("get and wait own bg state response for cycle %v", cycleId)
	cs := s.GetConsensusStateByCycleId(cycleId)

	op := NewGetAndWaitOwnBGStateResponseOp(s.BGID, cs)

	if !cs.AddOPandWait(op) {
		logger.Fatal("should return true get and wait own bg state response")
	}

	return op.GetResult()
}

// return the state response of its own BG for cycleId
func (s *Server) GetBGStateResponseForCycle(cycleId int) *pb.StateResponseMessage {
	logger.Debugf("get bg state response for cycle %v", cycleId)
	cs := s.GetConsensusStateByCycleId(cycleId)

	op := NewGetBGStateResponseOp(s.BGID, cs)

	if !cs.AddOPandWait(op) {
		logger.Fatal("should return true get bg state response")
	}

	return op.GetResult()
}

// return the cycle result until the cycle completes
func (s *Server) GetCycleResult(cycleId int) []*pb.StateResponseMessage {
	logger.Debugf("Get cycle result for cycle %v", cycleId)
	if cycleId <= s.round2LastFinishedCycle-common.MaxCycleNumberInStorage {
		panic("name-removed: this cycle has been overwritten!")
	}

	cs := s.GetConsensusStateByCycleId(cycleId)
	common.AssertTrue(cs.cycleId == cycleId, "Cycle IDs should match!", logger)
	csWait := s.GetConsensusStateByCycleId(cycleId + int(s.config.GetMaxPipelineDepth()))
	common.AssertTrue(csWait.cycleId == cycleId+int(s.config.GetMaxPipelineDepth()), "JustWait Cycle IDs should match!", logger)

	op := NewWaitCycleComplete(cs)
	opJustWait := NewJustWaitCycleComplete(csWait)

	if !cs.AddOPandWait(op) {
		logger.Fatal("should return true get cycle result")
	}

	// name-removed: wait until (cycleId + depth) completes
	if !cs.AddOPandWait(opJustWait) {
		logger.Fatal("should return true get cycle result")
	}

	return op.GetResult()
}

// send start cycle requst to BG leader
// TODO: start cycle request should have a proof
// only the request that have a valid proof can trigger to start a cycle
func (s *Server) StartCycle(stateRequest *pb.StateRequestMessage) {
	if s.IsLeader() {
		// if the node is monitor then process the start cycle request
		s.requestManager.AddSkipCycle(SkipCycleApplicationMaterial{
			skipTo:      uint32(stateRequest.CycleId),
			BGApplicant: uint32(stateRequest.BgId),
			view:        stateRequest.View,
			sequence:    stateRequest.Sequence,
			proof:       stateRequest.Proof,
			signedOn:    stateRequest.Hash,
		})
		return
	}
	// otherwise, send the state request to monitor
	s.sendStateRequestToMonitor(stateRequest)
}

func (s *Server) sendStateRequestToMonitor(sr *pb.StateRequestMessage) {
	logger.Debugf("Send state request from server %v bg %v of cycle %v to monitor since state response for bg %v is not ready", sr.SenderId, sr.BgId, sr.CycleId, s.BGID)
	dstAddr := s.GetLeader()
	conn := s.GetSelfBGInfo().GetConnection(dstAddr)
	conn.AddEvent(NewStateRequestToMonitorEvent(sr, 2*time.Second, s))
}

func (s *Server) processSBFTResult() {

	for {
		// blocking until next cycle result is received from SBFT replica
		bftResult, senderIds := s.resultReceiver.GetNextCycle()
		// name-removed: this is in order
		s.round2LastFinishedCycle = int(bftResult.CurrentCycle)
		logger.Debugf("Current cycle %d, txn sender Ids %v", bftResult.CurrentCycle, senderIds)
		s.reassembleTxn(bftResult)

		bftResult.Hash = make([]byte, 8, 40)
		binary.LittleEndian.PutUint32(bftResult.Hash[0:4], bftResult.LastCycle)
		binary.LittleEndian.PutUint32(bftResult.Hash[4:8], bftResult.CurrentCycle)
		bftResult.Hash = append(bftResult.Hash, util.ComputeBftResultHash(bftResult)...)

		if s.config.IsHeaderBodySeparate() && bftResult.FullTxnList == nil {
			logger.Fatal("Txn bodies are not attached")
		}

		// Unpack SkipCycle results

		// Last cycle is not empty
		if bftResult.CurrentCycle-bftResult.LastCycle > 1 {
			logger.Warningf("Unpacking cycle %d.", bftResult.CurrentCycle)
		}

		stateResponse := &pb.StateResponseMessage{
			CycleId:  int32(bftResult.CurrentCycle),
			SenderId: s.ID,
			BgId:     int32(s.BGID),
			Result:   bftResult,
		}
		stateResponse.Result.ReplaceMsgListWithHash = false

		s.raftLayer.HandleOwnStateResponse(stateResponse)

		emptyResult := pb.BftResult{
			View:                   bftResult.View,
			Sequence:               bftResult.Sequence,
			Nonce:                  bftResult.Nonce,
			LastCycle:              bftResult.LastCycle,
			CurrentCycle:           bftResult.CurrentCycle,
			MessageList:            nil,
			Hash:                   bftResult.Hash,
			CommitProof:            bftResult.CommitProof,
			BodyAttached:           false,
			FullTxnList:            nil,
			ReplaceMsgListWithHash: true,
		}

		// Empty cycles
		if bftResult.CurrentCycle-bftResult.LastCycle > 1 {
			logger.Warningf("This BFT result has cycle %d to %d", bftResult.LastCycle+1, bftResult.CurrentCycle)
		}
		for cycleID := bftResult.LastCycle + 1; cycleID < bftResult.CurrentCycle; cycleID++ {
			logger.Warningf("Unpacking cycle %d.", cycleID)
			stateResponse := &pb.StateResponseMessage{
				CycleId:  int32(cycleID),
				SenderId: s.ID,
				BgId:     int32(s.BGID),
				Result:   &emptyResult,
			}

			s.raftLayer.HandleOwnStateResponse(stateResponse)
		}

		// check if the list of txn is sent by this super leaf
		for _, txnSenderId := range senderIds {
			if txnSenderId[:4] == s.ID[:4] {
				logger.Debugf("cycle %v is partially sent by server %v", stateResponse.CycleId, s.ID)
				s.requestManager.previousCycleComplete <- true
			}
		}
	}
}

func (s *Server) forwardTxnToLeader(txn *pb.TransactionMessage) {
	dstAddr := s.GetLeader()
	conn := s.GetSelfBGInfo().GetConnection(dstAddr)
	conn.AddEvent(NewTxnMsgToMonitorEvent(txn, 2*time.Second, s))
}

func (s *Server) forwardTxnBodyToLeader(txn *pb.TxnBodyMessage) {
	dstAddr := s.GetLeader()
	logger.Debugf("forward txn body to leader: txn num %v from server %v", len(txn.TxnMessageBlock.TxnMessageList), txn.SenderId)
	conn := s.GetSelfBGInfo().GetConnection(dstAddr)
	conn.AddEvent(NewTxnBodyToMonitorEvent(txn, 2*time.Second, s))
}

func (s *Server) reassembleTxn(bftResult *pb.BftResult) {
	bftResult.BodyAttached = s.config.IsHeaderBodySeparate()
	if s.config.IsHeaderBodySeparate() {
		bftResult.FullTxnList = make([]*pb.TxnMessageBlock, 0)
		for _, msg := range bftResult.MessageList {
			txnList := make([]*pb.TransactionMessage, 0)
			for _, header := range msg.TxnHeaderBlock.TxnHeaderList {
				txn := s.txnStore.GetBodyRequest(header.HashContent)
				txnList = append(txnList, txn)
			}

			orderedMessage := &pb.TxnMessageBlock{
				TxnMessageList: txnList,
			}

			bftResult.FullTxnList = append(bftResult.FullTxnList, orderedMessage)
		}
	}
}
