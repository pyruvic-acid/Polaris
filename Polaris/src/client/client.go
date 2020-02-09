package client

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"runtime"

	"Polaris/src/byzantineGroup"
	"Polaris/src/configuration"
	pb "Polaris/src/rpc"
	"Polaris/src/transaction"
	"time"

	"github.com/op/go-logging"
)

var logger = logging.MustGetLogger("client")

type Client struct {
	ID           string
	localServers []string

	BGInfo *byzantineGroup.ByzantineGroup

	nextCycle int

	config *configuration.Configuration

	connForResult  map[string]*byzantineGroup.Connection
	connForSendTxn map[string]*byzantineGroup.Connection

	superleafNodes []string
}

func NewClient(configFilePath string, configHome string) *Client {
	client := &Client{
		localServers:   make([]string, 0),
		connForResult:  make(map[string]*byzantineGroup.Connection),
		connForSendTxn: make(map[string]*byzantineGroup.Connection),
		nextCycle:      1,
	}

	raw, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		logger.Fatal(err)
	}

	c := make(map[string]interface{})
	json.Unmarshal(raw, &c)

	client.ID = c["ID"].(string)

	client.BGInfo = byzantineGroup.NewByzantineGroup(c["bg"])

	for _, s := range c["local_servers"].([]interface{}) {
		client.localServers = append(client.localServers, s.(string))
	}

	//	for server, _ := range client.localServers {
	//		if client.BGInfo.IsLeader(server) {
	//			client.localServers[server] = true
	//		}
	//	}

	client.config = configuration.NewConfiguration(client.ID, configHome)

	client.superleafNodes = client.getSuperLeafLeaders()

	return client
}

func (c *Client) getConnectionForResult(dstAddr string) *byzantineGroup.Connection {
	if _, exist := c.connForResult[dstAddr]; !exist {
		c.connForResult[dstAddr] = c.BGInfo.GetNewConnection(dstAddr)
	}

	return c.connForResult[dstAddr]
}

func (c *Client) getConnectionForSendTxn(dstAddr string) *byzantineGroup.Connection {
	if _, exist := c.connForSendTxn[dstAddr]; !exist {
		c.connForSendTxn[dstAddr] = c.BGInfo.GetNewConnection(dstAddr)
	}

	return c.connForSendTxn[dstAddr]
}

func (c *Client) SendTxn(txnContent []byte, header []byte) {
	req := &pb.TransactionMessage{
		TxnContent: txnContent,
		TxnHeader:  header,
		SenderId:   c.ID,
	}

	addr := c.getLocalServerLeader()

	conn := c.getConnectionForSendTxn(addr)

	conn.AddEvent(
		NewSendTxnEvent(req, time.Second*time.Duration(10), addr),
	)
}

func (c *Client) GetNextCycle() []*transaction.PolarisTxn {
	requestCycle := &pb.RequestCycleMessage{
		SenderId: c.ID,
		CycleId:  int32(c.nextCycle),
	}

	//logger.Debugf("send request to get cycle result %v", c.nextCycle)

	//addrList := c.getSuperLeafLeaders()

	for _, addr := range c.superleafNodes {
		conn := c.getConnectionForResult(addr)
		timeout := 2
		if c.nextCycle == 1 {
			timeout = 10
		}
		e := NewGetCycleEvent(requestCycle, time.Second*time.Duration(timeout), addr)
		conn.AddEvent(e)
		cInfo := e.WaitAndGetResponse()

		if cInfo == nil {
			logger.Warningf("try to get cycle %v result from server %v but the result is nil try next server", c.nextCycle, addr)
			continue
		}

		count := 0
		for _, stateResponse := range cInfo.TxnList {
			if c.config.IsHeaderBodySeparate() {
				for _, txnMsgBlock := range stateResponse.Result.FullTxnList {
					count += len(txnMsgBlock.TxnMessageList)
				}
			} else {
				for _, slRequestMsg := range stateResponse.Result.MessageList {
					count += len(slRequestMsg.TxnHeaderBlock.TxnHeaderList)
				}
			}
		}

		txnList := make([]*transaction.PolarisTxn, count)
		count = 0

		for _, stateResponse := range cInfo.TxnList {
			if c.config.IsHeaderBodySeparate() {
				for slReqIndex, txnMsgBlock := range stateResponse.Result.FullTxnList {
					for i, t := range txnMsgBlock.TxnMessageList {
						txnList[count] = &transaction.PolarisTxn{
							Header:   stateResponse.Result.MessageList[slReqIndex].TxnHeaderBlock.TxnHeaderList[i].HashContent,
							Body:     t.TxnContent,
							SenderId: t.SenderId,
							CycleId:  int(cInfo.CycleId),
						}
						count++
					}
				}
			} else {
				for _, slReq := range stateResponse.Result.MessageList {
					for _, t := range slReq.TxnHeaderBlock.TxnHeaderList {
						txnList[count] = &transaction.PolarisTxn{
							Header:   []byte{},
							Body:     t.HashContent,
							SenderId: t.SenderId,
							CycleId:  int(cInfo.CycleId),
						}
						count++
					}
				}
			}
		}

		logger.Warningf("FLOG cycle %v txn num %v", c.nextCycle, len(txnList))
		if c.nextCycle%100 == 1 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			totalOccupied := (m.StackSys + m.HeapSys) / 1024 / 1024
			totalInUse := (m.HeapInuse + m.StackInuse) / 1024 / 1024
			logger.Warningf("Current in-use: %v MB, Current consumption: %v MB, %v GCs", totalInUse, totalOccupied, m.NumGC)
		}

		c.nextCycle++
		return txnList
	}

	logger.Errorf("cannot get cycle %v result from local BG servers %v", c.nextCycle, c.superleafNodes)
	return nil
}

func (c *Client) getLocalServerLeader() string {
	//for addr, isLeader := range c.localServers {
	//	if isLeader {
	//		return addr
	//	}
	//}
	i := rand.Intn(len(c.localServers))
	return c.localServers[i]
}

func (c *Client) getSuperLeafLeaders() []string {
	leaders := make([]string, 0)
	localLeader := c.getLocalServerLeader()
	leaders = append(leaders, localLeader)
	localSuperLeafId, _ := c.BGInfo.GetSuperLeafIdByServer(localLeader)
	x := localSuperLeafId % 4
	y := localSuperLeafId - x
	for i := 0; i < 4; i++ {
		slId := y + i
		if slId == localSuperLeafId {
			continue
		}
		l := c.BGInfo.GetRandomNode(slId)
		leaders = append(leaders, l)
	}
	logger.Debugf("leaders: %v", leaders)
	return leaders
}
