// +build !nosign

package server

import pb "Polaris/src/rpc"

func (s *Server) checkQC(sp *pb.StateResponseMessage) bool {
	r := s.resultVerifier.verifyResponse(sp)
	return r
}

func (s *Server) checkRequestQC(sp *pb.StateRequestMessage) bool {
	r := s.resultVerifier.verifyRequest(sp)
	return r
}
