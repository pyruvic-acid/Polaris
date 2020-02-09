// +build fixemu

package server

import (
	"math/rand"
	"time"
)

func (d *Dissemination) sendStateRequest(
	cId int,
	bgId int,
	emulatorList map[string][]string) {

	//BG := d.server.BGsInfo[bgId]
	req := d.buildStateRequest(cId)
	// list of emulator from different super-leaf
	// TODO: current the list is from a configuration file.
	// it should from the membership service

	// if the server already receives the state response from the BG
	// then do nothing
	if d.server.CheckBGState(cId, bgId) {
		logger.Debugf("Server %v already have state respnse from bg %v for cycle %v",
			d.server.ID, bgId, cId)
		// already get the state of bgId for cId
		return
	}

	emulatorPool := make([]string, 0)
	for SLID, eList := range emulatorList {
		if SLID == d.server.ID[2:3] {
			emulatorPool = append(emulatorPool, eList...)
		}
	}

	for !d.server.CheckBGState(cId, bgId) {
		// TODO: use the first one for now
		// randomly pick one of the emulator
		i := rand.Intn(len(emulatorPool))
		e := emulatorPool[i]

		logger.Warningf("send state request to server %v for cycle %v", e, cId)
		tcpAddr := d.server.serverTCPAddrMap[e]
		d.server.tcpManager.EncodeAndSend(tcpAddr, 1, req)

		timer := time.NewTimer(5 * time.Second)
		<-timer.C
	}

}
