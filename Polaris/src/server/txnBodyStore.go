package server

import (
	pb "Polaris/src/rpc"
	"fmt"
)

type GetBodyRequest struct {
	header []byte
	wait   chan bool
	txn    *pb.TransactionMessage
}

func NewGetBodyRequest(header []byte) *GetBodyRequest {
	return &GetBodyRequest{
		header: header,
		wait:   make(chan bool),
	}
}

func (r *GetBodyRequest) BlockOwner() bool {
	return <-r.wait
}

type TxnBodyStore struct {
	bodyChan           chan *pb.TransactionMessage
	getBodyRequestChan chan *GetBodyRequest

	headBodyMap     map[int]*pb.TransactionMessage
	waitBodyRequest map[int][]*GetBodyRequest
}

func NewTxnBodyStore() *TxnBodyStore {
	t := &TxnBodyStore{
		bodyChan:           make(chan *pb.TransactionMessage, 10240),
		getBodyRequestChan: make(chan *GetBodyRequest, 10240),
		headBodyMap:        make(map[int]*pb.TransactionMessage),
		waitBodyRequest:    make(map[int][]*GetBodyRequest),
	}
	go t.process()
	return t
}

func (ts *TxnBodyStore) PutTxnMsgBlock(txnBody *pb.TxnBodyMessage) {
	//logger.Warningf("Add body from server %v, txn num %v to store", txnBody.SenderId, len(txnBody.TxnMessageBlock.TxnMessageList))
	for _, txn := range txnBody.TxnMessageBlock.TxnMessageList {
		ts.bodyChan <- txn
	}
}

func (ts *TxnBodyStore) GetBodyRequest(header []byte) *pb.TransactionMessage {
	req := NewGetBodyRequest(header)
	ts.getBodyRequestChan <- req
	//logger.Debugf("wait to get result for request %v", req)
	if !req.BlockOwner() {
		logger.Fatal("get body request blockowner should return true")
	}
	//logger.Debugf("get result for request %v", req)
	return req.txn
}

func hash(k []byte) int {
	key := fmt.Sprintf("%s", k)
	h := 0
	s := len(k)
	for i := 0; i < len(key); i++ {
		h = s*h + int(key[i])
	}
	return h
}

func (ts *TxnBodyStore) process() {
	for {
		select {
		case txn := <-ts.bodyChan:
			//header := util.ComputeTxnHash(txn)
			header := txn.TxnHeader
			i := hash(header)
			if t, exist := ts.headBodyMap[i]; exist {
				logger.Debugf("header %v already exist txn %v", header, t)
			}
			ts.headBodyMap[i] = txn
			//logger.Debugf("add txn: header: %v, body: %v", header, txn)
			if waitRequests, exist := ts.waitBodyRequest[i]; exist {
				//logger.Warningf("wake up waiting request %v for header %v", waitRequests, header)
				for _, w := range waitRequests {
					w.txn = txn
					w.wait <- true
				}
				delete(ts.waitBodyRequest, i)
				//delete(ts.headBodyMap, header)
			}
		case req := <-ts.getBodyRequestChan:
			i := hash(req.header)
			if t, exist := ts.headBodyMap[i]; exist {
				req.txn = t
				req.wait <- true
				//delete(ts.headBodyMap, req.header)
			} else {
				//logger.Debugf("header %v does not exist, wait", req.header)
				if _, exist := ts.waitBodyRequest[i]; !exist {
					ts.waitBodyRequest[i] = make([]*GetBodyRequest, 0)
					//logger.Fatalf("header %v already has waiting request %v", req.header, r)
				}
				ts.waitBodyRequest[i] = append(ts.waitBodyRequest[i], req)
			}
		}
	}
}
