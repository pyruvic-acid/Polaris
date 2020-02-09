package server

import (
	//"Polaris/src/common"
	membership "Polaris/src/membership-service-rpc"
	pb "Polaris/src/rpc"
	"encoding/json"
	"fmt"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	//"google.golang.org/grpc/codes"
	//"google.golang.org/grpc/examples/go_client/membership"

	"strconv"
	"time"
)

const queueLen = 10240

// Dissemination struct is used for send the state request and
// handle the state response received from other BGs
type Dissemination struct {
	server *Server

	// state request to send
	stateRequestToSendChan chan int

	// state response received
	stateResponseChan chan *pb.StateResponseMessage

	emulators map[string]map[string][]string
}

func NewDissemination() *Dissemination {
	diss := &Dissemination{
		server:                 nil,
		stateRequestToSendChan: make(chan int, queueLen),
		stateResponseChan:      make(chan *pb.StateResponseMessage, queueLen),
		emulators:              make(map[string]map[string][]string),
	}

	return diss
}

func (d *Dissemination) SetServer(server *Server) {
	d.server = server
}

func (d *Dissemination) setEmulator() {
	for id, bg := range d.server.BGsInfo {
		if id == d.server.BGID {
			continue
		}
		emulators := bg.GetEmulator()
		idStr := fmt.Sprint(id)
		if _, exist := d.emulators[idStr]; !exist {
			d.emulators[idStr] = make(map[string][]string)
		}

		for superLeaf, servers := range emulators {
			slIdStr := fmt.Sprint(superLeaf)
			if _, exist := d.emulators[idStr][slIdStr]; !exist {
				d.emulators[idStr][slIdStr] = make([]string, len(servers))
			}
			for i, s := range servers {
				d.emulators[idStr][slIdStr][i] = s
			}
		}
	}
	logger.Warningf("emulators: %v", d.emulators)
}

func (d *Dissemination) Start() {
	d.process()
}

// single thread to handle the requests
func (d *Dissemination) process() {
	for {
		select {
		case cId := <-d.stateRequestToSendChan:
			d.sendStateRequestsToBGs(cId)
		case sp := <-d.stateResponseChan:
			d.sendStateResponseToMonitor(sp)
		}
	}
}

type memberReqeuest struct {
	Mode   string `json:"Mode"`
	SlId   string `json:"SLID"`
	Height string `json:"height"`
}

// return: BGID -> SLID -> list of emulators
func (d *Dissemination) getEmulater(cycle int) map[string]map[string][]string {
	// Ask the membership service to get the emulator every 100 cycles
	if cycle%500 != 0 && len(d.emulators) != 0 {
		return d.emulators
	}

	slid := fmt.Sprintf("%v-%v", d.server.BGID+1, d.server.SuperLeafID+1)
	logger.Debugf("get emulator, own superleaf id: %v", slid)
	reqObj := memberReqeuest{"emulators", slid, "2"}
	reqJson, err := json.Marshal(reqObj)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Debugf(string(reqJson))
	req := string(reqJson)

	logger.Debugf("get emulator request %v for superleaf %v", req, slid)

	//ctx , cancel := context.With

	conn, err := grpc.Dial(d.server.SBFTClientAddressForMembership, grpc.WithInsecure())
	if err != nil {
		logger.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client := membership.NewMemRequestClient(conn)

	r, err := client.SayHello(ctx, &membership.RequestMembership{Name: req, ReadOnly: 1})
	if err != nil {
		logger.Fatalf("could not connect: %v", err)
	}

	logger.Debugf("get emulator result %v for superleaf %v", r, slid)
	c := make(map[string]interface{})
	err = json.Unmarshal([]byte(r.Message), &c)

	if err != nil {
		logger.Fatalf("cannot parse the result from membership service")
	}

	// TODO: should check the certificate
	d.emulators = make(map[string]map[string][]string)
	for bgId, slInter := range c {
		sl := slInter.(map[string]interface{})
		d.emulators[bgId] = make(map[string][]string)
		for slId, member := range sl {
			temp := member.(map[string]interface{})
			ips := temp["ip"].([]interface{})

			d.emulators[bgId][slId] = make([]string, len(ips))
			for i, ipInter := range ips {
				ip := ipInter.(string)
				d.emulators[bgId][slId][i] = ip
			}
		}
	}
	return d.emulators
}

// send the state requests to emulators of other BGs
func (d *Dissemination) sendStateRequestsToBGs(cId int) {

	//emulators := d.getEmulater(cId)
	if len(d.emulators) == 0 {
		d.setEmulator()
	}
	emulators := d.emulators
	// send the state request to one emulator for each super-leaf in the BG
	for bgID, emulatorList := range emulators {
		// skip its own BG
		bgIDInt, err := strconv.Atoi(bgID)
		if err != nil {
			logger.Fatalf("cannot convert bgId to int %v", err)
		}
		//	bgIDInt -= 1
		if bgIDInt == d.server.BGID {
			continue
		}
		// send the different BG in parallel
		go d.sendStateRequest(cId, bgIDInt, emulatorList)
	}
}

//type ThirdRoundError struct {
//	Msg  string
//	Code int
//}

func (d *Dissemination) handleStateRequest(stateRequest *pb.StateRequestMessage) {
	logger.Warningf("receive state request from %v cycle %v", stateRequest.SenderId, stateRequest.CycleId)
	r := d.server.CheckStateRequest(stateRequest)
	if !r {
		//errorMsg := &ThirdRoundError{
		//	Msg:  "invalid state response",
		//	Code: common.InvalidStateResponse,
		//}

		//d.server.tcpmgr.EncodeAndSend(stateRequestMessage.SenderAddr, 'E', errorMsg)
		logger.Fatalf("invalid state response from server %v cycle %v", stateRequest.SenderId, stateRequest.CycleId)
		return
		// send the error
	}

	stateResponseMessage := d.server.GetOrWaitBGStateResponse(stateRequest)

	logger.Debugf("Get state response for cycle %v; Check if it is nil", stateRequest.CycleId)

	if stateResponseMessage == nil {
		logger.Warningf("State Response for BG %v Cycle %v is not ready!!!!! Server %v add pending request",
			d.server.BGID, stateRequest.CycleId, stateRequest.SenderId)
		// when the state response is not ready
		// send the null response, and send response back when it is ready
		// by calling StateResponse RPC
		d.server.StartCycle(stateRequest)

		//	errMsg := fmt.Sprintf("state response for bg %v cycle %v is not ready",
		//		d.server.BGID, stateRequest.CycleId)
		//	errorMsg := &ThirdRoundError{
		//		Msg:  errMsg,
		//		Code: common.StateResponseNotReady,
		//	}
		//	// send the error
		//	d.server.tcpmgr.EncodeAndSend(stateRequestMessage.SenderAddr, 'E', errorMsg)
		return
	}

	logger.Debugf("Send the state respnose back to server %v for cycle %v",
		stateRequest.SenderId, stateRequest.CycleId)

	// send the state response
	tcpAddr := d.server.serverTCPAddrMap[stateRequest.SenderAddr]
	d.server.tcpManager.EncodeAndSend(tcpAddr, 2, stateResponseMessage)

}

func (d *Dissemination) handleStateResponse(stateResponse *pb.StateResponseMessage) {
	logger.Warningf("Receive the state respnose from server %v for cycle %v", stateResponse.SenderId, stateResponse.CycleId)
	r := d.server.checkQC(stateResponse)
	if !r {
		logger.Fatalf("state response invalid from %v cycle %v", stateResponse.SenderId, stateResponse.CycleId)
		return
	}
	if !d.server.CheckBGState(int(stateResponse.CycleId), int(stateResponse.BgId)) {
		d.stateResponseChan <- stateResponse
	}
}

// receive the state response from other bg
// send to the monitor to replicate to other super-leaf members
func (d *Dissemination) sendStateResponseToMonitor(sp *pb.StateResponseMessage) {
	logger.Debugf("Send the state response from bg %v of cycle %v to monitor", sp.BgId, sp.CycleId)
	// if the server is Monitor, then directly add to the channel
	if d.server.IsLeader() {
		d.server.raftLayer.AddStateResponse(sp)
	} else {
		// otherwise, send the request to the Monitor
		dstAddr := d.server.GetLeader()
		conn := d.server.BGsInfo[d.server.BGID].GetConnection(dstAddr)
		conn.AddEvent(NewStateResponseToMonitorEvent(sp, 5*time.Second, d.server))
	}
}

// Add the cycleId to stateRequestToSendChan
func (d *Dissemination) AddStateRequestForCycle(cycleId int) {
	logger.Debugf("add cycle %v to send the state request", cycleId)
	d.stateRequestToSendChan <- cycleId
}

// Add the state response to to the stateResponseChan
func (d *Dissemination) AddStateResponse(sp *pb.StateResponseMessage) {
	d.stateResponseChan <- sp
}

// create state request message
func (d *Dissemination) buildStateRequest(cycleId int) *pb.StateRequestMessage {

	// wait until its own state response for cycleId is replicated by Raft
	stateResponse := d.server.GetAndWaitOwnBGStateResponse(cycleId)

	stateRequestMessage := &pb.StateRequestMessage{
		CycleId:    int32(cycleId),
		BgId:       int32(d.server.BGID),
		SenderId:   d.server.ID,
		SenderAddr: d.server.addr,
		Sequence:   stateResponse.Result.Sequence,
		View:       stateResponse.Result.View,
		Proof:      stateResponse.Result.CommitProof,
		Hash:       stateResponse.Result.Hash,
	}

	return stateRequestMessage
}
