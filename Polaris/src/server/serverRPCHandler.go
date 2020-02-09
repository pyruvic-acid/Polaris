package server

import (
	"Polaris/src/common"
	pb "Polaris/src/rpc"

	//"Polaris/src/util"
	"fmt"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

///////////////////////////////////////////////////////

// Client RPC functions
func (s *Server) AcceptTransaction(ctx context.Context, txn *pb.TransactionMessage) (*pb.Empty, error) {
	if !s.IsLeader() {
		s.forwardTxnToLeader(txn)
		return &pb.Empty{}, nil
	}
	//logger.Debugf("receive txn from client %v, body: %v, header %v", txn.SenderId, txn, txn.TxnHeader)
	s.requestManager.incomingTxnChan <- txn
	return &pb.Empty{}, nil
}

func (s *Server) GetCycle(ctx context.Context, msg *pb.RequestCycleMessage) (*pb.CycleInfo, error) {
	logger.Debugf("Receive get cycle result request from client %v for cycle %v", msg.SenderId, msg.CycleId)
	res := s.GetCycleResult(int(msg.CycleId))
	logger.Debugf("send the cycle result back to client %v cycle %v", msg.SenderId, msg.CycleId)
	return &pb.CycleInfo{CycleId: msg.CycleId, TxnList: res}, nil
}

//////////////////////////////////////////////////////////
// forward txn message to leader
func (s *Server) TxnMessageToMonitor(ctx context.Context, msg *pb.TransactionMessage) (*pb.LeaderIdReply, error) {
	if !s.IsLeader() {
		logger.Debugf("server %v is not raft leader", s.ID)
		return nil, grpc.Errorf(common.NotLeaderErrorCode, s.GetLeader())
	}
	//logger.Debugf("TxnMessageToMonitor from server %v client %v txn body %v header %v", msg.SenderId, msg, msg.TxnHeader)

	s.requestManager.incomingTxnChan <- msg
	return &pb.LeaderIdReply{Leader: s.GetLeader()}, nil
}

func (s *Server) TxnBodyToMonitor(ctx context.Context, msg *pb.TxnBodyMessage) (*pb.LeaderIdReply, error) {
	if !s.IsLeader() {
		logger.Debugf("server %v is not raft leader", s.ID)
		return nil, grpc.Errorf(common.NotLeaderErrorCode, s.GetLeader())
	}
	logger.Warningf("TxnBodyToMonitor from server %v, txn num %v", msg.SenderId, len(msg.TxnMessageBlock.TxnMessageList))

	s.raftLayer.txnBodyChan <- msg
	return &pb.LeaderIdReply{Leader: s.GetLeader()}, nil
}

func (s *Server) AcceptTxnBody(ctx context.Context, txn *pb.TxnBodyMessage) (*pb.Empty, error) {
	if !s.IsLeader() {
		//logger.Debugf("server %v is not raft leader", s.ID)
		//return nil, grpc.Errorf(common.NotLeaderErrorCode, s.GetLeader())
		s.forwardTxnBodyToLeader(txn)
		return &pb.Empty{}, nil
	}

	logger.Warningf("AcceptTxnMessageBlock from server %v, txn num %v", txn.SenderId, len(txn.TxnMessageBlock.TxnMessageList))
	// replicate the body to the members in the super-leaf
	s.raftLayer.txnBodyChan <- txn
	return &pb.Empty{}, nil
}

////////////////////////////////////////////////////////////
// dissenmination layer RPC
func (s *Server) StateRequest(ctx context.Context, req *pb.StateRequestMessage) (*pb.StateResponseMessage, error) {
	logger.Warningf("Receive state request from server %v for cycle %v", req.SenderId, req.CycleId)

	// check state response. Only accept state requst when the state response is valid
	r := s.CheckStateRequest(req)

	if !r {
		errMsg := fmt.Sprintf("state request from bg %v cycle %v server %v is invalid",
			req.BgId, req.CycleId, req.SenderId)
		return nil, grpc.Errorf(common.InvalidStateResponse, errMsg)
	}

	stateResponseMessage := s.GetOrWaitBGStateResponse(req)

	logger.Debugf("Get state response for cycle %v; Check if it is nil", req.CycleId)

	if stateResponseMessage == nil {
		logger.Debugf("State Response for BG %v Cycle %v is not ready!!!!! Server %v add pending request",
			s.BGID, req.CycleId, req.SenderId)
		// when the state response is not ready
		// send the null response, and send response back when it is ready
		// by calling StateResponse RPC
		s.StartCycle(req)
		errMsg := fmt.Sprintf("state response for bg %v cycle %v is not ready",
			s.BGID, req.CycleId)
		return nil, grpc.Errorf(common.StateResponseNotReady, errMsg)
	}

	logger.Debugf("Send the state respnose back to server %v for cycle %v",
		req.SenderId, req.CycleId)
	return stateResponseMessage, nil
}

func (s *Server) StateRequestToMonitor(ctx context.Context, msg *pb.StateRequestMessage) (*pb.LeaderIdReply, error) {
	if !s.IsLeader() {
		logger.Debugf("server %v is not raft leader", s.ID)
		return nil, grpc.Errorf(common.NotLeaderErrorCode, s.GetLeader())
	}

	// check if this server's stateresponse for CycleId is exist
	res := s.CheckBGState(int(msg.CycleId), s.BGID)

	logger.Debugf("Monitor Check state response for cycle %v; Check if it is nil", msg.CycleId)

	if !res {
		logger.Debugf("Monitor State Response for BG %v Cycle %v is not ready!!!!! Start cycle",
			s.BGID, msg.CycleId)
		// when the state response is not ready
		// send the null response, and send response back when it is ready
		// by calling StateResponse RPC
		s.StartCycle(msg)
	}

	return &pb.LeaderIdReply{Leader: s.GetLeader()}, nil
}

func (s *Server) StateResponse(ctx context.Context, msg *pb.StateResponseMessage) (*pb.Empty, error) {

	r := s.checkQC(msg)

	if !r {
		errMsg := fmt.Sprintf("state response from bg %v cycle %v server %v is invalid",
			msg.BgId, msg.CycleId, msg.SenderId)
		return nil, grpc.Errorf(common.InvalidStateResponse, errMsg)
	}

	logger.Debugf("Receive State Response from BG %v cycle %v", msg.BgId, msg.CycleId)
	if !s.CheckBGState(int(msg.CycleId), int(msg.BgId)) {
		s.disseminationLayer.AddStateResponse(msg)
	}

	return &pb.Empty{}, nil
}

func (s *Server) StateResponseToMonitor(ctx context.Context, msg *pb.StateResponseMessage) (*pb.LeaderIdReply, error) {

	if !s.IsLeader() {
		logger.Debugf("server %v is not raft leader", s.ID)
		return nil, grpc.Errorf(common.NotLeaderErrorCode, s.GetLeader())
	}

	s.raftLayer.AddStateResponse(msg)

	return &pb.LeaderIdReply{Leader: s.GetLeader()}, nil
}
