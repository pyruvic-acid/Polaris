// +build nosign

package server

import pb "Polaris/src/rpc"

func (s *Server) checkQC(sp *pb.StateResponseMessage) bool {
	return true
}

func (s *Server) checkRequestQC(sp *pb.StateRequestMessage) bool {
	return true
}
