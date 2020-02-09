package transaction

import logging "github.com/op/go-logging"

// This is an interface
// TODO: negotiate interface with client side

var logger = logging.MustGetLogger("server.txn")

type PolarisTxn struct {
	Header   []byte
	Body     []byte
	SenderId string
	CycleId  int
}
