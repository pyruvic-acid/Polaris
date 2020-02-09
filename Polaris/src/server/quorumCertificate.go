package server

//import "fmt"

type QuorumCertificate interface {
	GetOrderedTransaction()
	CheckSignatures() bool
}

type PBFTQuorumCertificate struct {
}

func (pq *PBFTQuorumCertificate) GetOrderedTransaction() {

}

func (pq *PBFTQuorumCertificate) CheckSignatures() bool {
	return true
}
