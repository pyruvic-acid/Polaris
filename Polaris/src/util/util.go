package util

import (
	"crypto/sha256"
	"fmt"

	pb "Polaris/src/rpc"

	"github.com/gogo/protobuf/proto"
	"github.com/op/go-logging"
)

var logger = logging.MustGetLogger("util")

func ComputeTxnHash(txn []byte) string {
	h := sha256.New()

	h.Write(txn)
	logger.Debugf("sha256hash txn header = %v", h.Sum(nil))
	header := fmt.Sprintf("%x", h.Sum(nil))
	return header
}

func xorBytes(a, b []byte) ([]byte, error) {
	if len(a) != len(b) {
		return nil, fmt.Errorf("length of byte slices is not equivalent: %d != %d", len(a), len(b))
	}

	buf := make([]byte, len(a))

	for i := range a {
		buf[i] = a[i] ^ b[i]
	}

	return buf, nil
}

func ComputeSLRequestHash(req *pb.SLRequestMessage) []byte {
	h := sha256.New()
	out, err := proto.Marshal(req)
	if err != nil {
		logger.Fatalf("Failed to encode :", err)
	}

	h.Write(out)
	return h.Sum(nil)
}

func ComputeByteHash(b []byte) []byte {
	h := sha256.New()
	h.Write(b)
	return h.Sum(nil)
}

func ComputeBftResultHash(res *pb.BftResult) []byte {
	first := true
	var hash []byte
	for _, msg := range res.MessageList {
		tmpHash := ComputeSLRequestHash(msg)

		if first {
			hash = tmpHash
			first = false
		} else {
			hash, _ = xorBytes(hash, tmpHash)
		}
	}

	finalHash := ComputeByteHash(hash)
	return finalHash
}

func CountNumberOfTxnInBftResult(res *pb.BftResult) int {
	count := 0
	for _, msg := range res.MessageList {
		count += len(msg.TxnHeaderBlock.TxnHeaderList)
	}
	return count
}
