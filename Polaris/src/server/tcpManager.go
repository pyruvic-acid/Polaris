package server

import (
	"bytes"
	"net"

	pb "Polaris/src/rpc"
	"Polaris/src/tlv"
	"encoding/gob"
	"io"
	"time"
	//"github.com/op/go-logging"
)

//var logger = logging.MustGetLogger("TCP Manager")

type TCPManager struct {
	outgoingConnections map[string]*net.TCPConn
	listener            *net.TCPListener
	incomingConnections []*net.TCPConn
	addAddr             *net.TCPAddr
	server              *Server
	codec               *tlv.Codec
}

// localAddr e.g. "127.0.0.1"
// portToListenOn e.g. "5588"
func NewTCPManager(portToListenOn string) *TCPManager {
	tcpmgr := &TCPManager{
		outgoingConnections: make(map[string]*net.TCPConn),
		listener:            nil,
		incomingConnections: make([]*net.TCPConn, 0),
		// Add more fields here
		codec: &tlv.Codec{TypeBytes: tlv.TwoBytes, LenBytes: tlv.FourBytes},
	}

	var err error
	//localAddrAndPort := localAddr + ":" + portToListenOn
	tcpmgr.addAddr, err = net.ResolveTCPAddr("tcp", ":"+portToListenOn)
	tcpmgr.listener, err = net.ListenTCP("tcp", tcpmgr.addAddr)

	if err != nil {
		logger.Fatalf("Failed to create TCP listener. Error = %v", err)
	}

	return tcpmgr
}

func (tcpmgr *TCPManager) Start() {
	go tcpmgr.listenThread()
}

func (tcpmgr *TCPManager) SetServer(server *Server) {
	tcpmgr.server = server
}

func (tcpmgr *TCPManager) GetConnction(addr string) *net.TCPConn {
	return tcpmgr.outgoingConnections[addr]
}

func (mgr *TCPManager) DialAll(rgstrRemoteIPAndPort []string) {
	//localIPWithBlankPort := net.ResolveTCPAddr("tcp", mgr.localIPAddr)
	//localIPWithBlankPort.Port = 0

	for _, addr := range rgstrRemoteIPAndPort {
		remoteAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			logger.Errorf("Cannot resolove the addr err %v", err)
		}

		for {
			conn, err := net.DialTCP("tcp", nil, remoteAddr)

			if err != nil {
				logger.Errorf("Failed to create TCP connection. Error = %v try again", err)
				timer := time.NewTimer(time.Second)
				<-timer.C
				continue
			}

			conn.SetKeepAlive(true)
			conn.SetNoDelay(true)
			mgr.outgoingConnections[addr] = conn
			break
		}
	}
}

// This function does not return
func (mgr *TCPManager) listenThread() {
	for {
		conn, err := mgr.listener.AcceptTCP()

		if err != nil {
			logger.Warningf("Failed to accept TCP connection. Error = %v", err)
		}

		conn.SetKeepAlive(true)
		conn.SetNoDelay(true)

		go mgr.handleConnection(conn)
		mgr.incomingConnections = append(mgr.incomingConnections, conn)
	}
}

func (mgr *TCPManager) EncodeAndSend(dstAddr string, typ uint, item interface{}) {
	conn := mgr.outgoingConnections[dstAddr]
	var payload bytes.Buffer

	enc := gob.NewEncoder(&payload)

	err := enc.Encode(item)
	if err != nil {
		logger.Fatalf("Cannot encode data payload. Error = %v", err)
	}
	logger.Debugf("Send msg %v to server %v payload size %v", typ, dstAddr, payload.Len())

	buf := new(bytes.Buffer)
	wr := tlv.NewWriter(buf, mgr.codec)
	record := &tlv.Record{
		Payload: payload.Bytes(),
		Type:    typ,
	}
	wr.Write(record)

	_, err = conn.Write(buf.Bytes())
	if err != nil {
		logger.Warningf("Error when writing to TCP socket. Error = %v", err)
	}
}

func (mgr *TCPManager) readAll(conn *net.TCPConn) *tlv.Record {
	headerBuf := make([]byte, 6)
	record := &tlv.Record{}

	var recvsize int
	var err error

	const HeaderSize = 6

	for offset := uint(0); offset < HeaderSize; {
		recvsize, err = conn.Read(headerBuf[offset:])
		logger.Debugf("read size %v", recvsize)

		if err != nil && err != io.EOF {
			logger.Fatalf("1st read error: err = %v, %d bytes read", err, recvsize)
		}

		// To make me feel more confident when casting it to uint
		if recvsize < 0 {
			logger.Fatalf("(negative!) %d bytes read", recvsize)
		}
		offset += uint(recvsize)

		if err == io.EOF {
			if offset < HeaderSize {
				logger.Fatal("1st read error -- EOF too early")
			} else {
				// It would break anyway
				break
			}
		}
	}

	reader := bytes.NewReader(headerBuf)
	tlvReader := tlv.NewReader(reader, mgr.codec)
	typ, length, err := tlvReader.GetTypeAndLen()
	if err != nil {
		logger.Fatalf("cannot get len and type err %v", err)
	}
	record.Type = typ
	record.Payload = make([]byte, length)

	for offset := uint(0); offset < length; offset += uint(recvsize) {
		recvsize, err = conn.Read(record.Payload[offset:])
		if err != nil {
			if err != io.EOF {
				logger.Fatalf("Subsequent read error: err = %v, %d bytes read", err, recvsize)
			}
			break
		}

		// To make me feel more confident when casting it to uint
		if recvsize < 0 {
			logger.Fatalf("(negative!) %d bytes read", recvsize)
		}

		logger.Debugf("read size %v", recvsize)
	}

	logger.Debugf("total size: %v, type %v", len(record.Payload), record.Type)
	return record
}

func (mgr *TCPManager) handleConnection(conn *net.TCPConn) {
	for {
		recode := mgr.readAll(conn)
		if len(recode.Payload) == 0 {
			continue
		}

		buffer := bytes.NewBuffer(recode.Payload)

		dec := gob.NewDecoder(buffer)

		switch recode.Type {
		case 1:
			var stateRequest *pb.StateRequestMessage
			err := dec.Decode(&stateRequest)
			if err != nil {
				logger.Errorf("Failed to decode stateRequest packet. %v", err)
			}
			logger.Warningf("Receive state request from server %v for cycle %v",
				stateRequest.SenderId, stateRequest.CycleId)
			go mgr.server.disseminationLayer.handleStateRequest(stateRequest)
		case 2:
			var stateResponse *pb.StateResponseMessage
			err := dec.Decode(&stateResponse)
			if err != nil {
				logger.Errorf("Failed to decode stateResponse packet. %v", err)
			}
			logger.Warningf("Receive state response from server %v for cycle %v",
				stateResponse.SenderId, stateResponse.CycleId)
			go mgr.server.disseminationLayer.handleStateResponse(stateResponse)

		default:
			logger.Debugf("Unrecognized packet format.")
		}

		// Take out the tag
		// Add more tags if you need to
		//		if byteBuffer[0] == byte('Q') {
		//			var stateRequest *pb.StateRequestMessage
		//			err := dec.Decode(&stateRequest)
		//			if err != nil {
		//				logger.Warning("Failed to decode packet.")
		//			}
		//			logger.Warningf("Receive state request from server %v for cycle %v", stateRequest.SenderId, stateRequest.CycleId)
		//			go mgr.server.disseminationLayer.handleStateRequest(stateRequest)
		//
		//			// You need to figure out the identity of the remote machine
		//			// from the payload
		//			// Find the outgoing connection
		//			// Prepare your response
		//			// Then send it with TCPManager.EncodeAndSend
		//		} else if byteBuffer[0] == byte('A') {
		//			var stateResponse *pb.StateResponseMessage
		//			err := dec.Decode(&stateResponse)
		//			if err != nil {
		//				logger.Warning("Failed to decode packet.")
		//			}
		//			logger.Warningf("Receive state response from server %v for cycle %v", stateResponse.SenderId, stateResponse.CycleId)
		//			go mgr.server.disseminationLayer.handleStateResponse(stateResponse)
		//			// You need to figure out the identity of the remote machine
		//			// from the payload
		//			// Find the outgoing connection
		//			// Prepare your response
		//			// Then send it with TCPManager.EncodeAndSend
		//			//	} else if byteBuffer[0] == byte('E') {
		//			//			// receive error message
		//			//			var err *ThirdRoundError
		//			//			err = dec.Decode(&error)
		//			//			go server.disseminationLayer.handleError(err)
		//			//
		//		} else {
		//			logger.Warning("Unrecognized packet format.")
		//
		//		}
	}
}
