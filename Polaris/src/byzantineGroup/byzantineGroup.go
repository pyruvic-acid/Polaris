package byzantineGroup

import (
	"github.com/op/go-logging"
	//"google.golang.org/grpc"
	"math/rand"
	"sync"
)

var logger = logging.MustGetLogger("byzantine group")

const queueLen = 10240

// ByzantineGroup contains the informations about the BG
// including: server address of superleafs,
//            leaders of superleafs,
//            gRPC connections of servers in the BG,
//            superleafId to serverId mapping
type ByzantineGroup struct {
	leaders     []string // superleafId -> leader addr
	leadersLock sync.RWMutex

	// connection object handls grpc connections of superleaf members
	// servers     []*Connection // superLeafId -> connection
	servers     []map[string]*Connection // superLeafId -> (server addr -> connection)
	serversLock sync.RWMutex

	//serversRPCConnection     []map[string]*grpc.ClientConn // superLeafId -> (server addr -> connection)
	//serversRPCConnectionLock sync.RWMutex

	serverToSuperLeafMap map[string]int // server addr -> superLeafId

	superLeafToServers map[int][]string // superLeafId -> list of server addr
}

func (b *ByzantineGroup) GetEmulator() map[int][]string {
	return b.superLeafToServers
}

func (b *ByzantineGroup) GetServerList() []string {
	sList := make([]string, 0)
	for s, _ := range b.serverToSuperLeafMap {
		sList = append(sList, s)
	}
	return sList
}

func parse2DString(i interface{}) [][]string {
	var s [][]string
	for _, r := range i.([]interface{}) {
		var row []string
		for _, e := range r.([]interface{}) {
			row = append(row, e.(string))
		}
		s = append(s, row)
	}
	return s
}

func NewByzantineGroup(serversI interface{}) *ByzantineGroup {
	servers := parse2DString(serversI)

	bg := &ByzantineGroup{
		serverToSuperLeafMap: make(map[string]int),
		superLeafToServers:   make(map[int][]string),
	}

	for i, c := range servers {
		bg.leaders = append(bg.leaders, "")
		//bg.servers = append(bg.servers, nil)
		bg.servers = append(bg.servers, make(map[string]*Connection))
		//bg.serversRPCConnection = append(bg.serversRPCConnection, make(map[string]*grpc.ClientConn))
		bg.superLeafToServers[i] = make([]string, 0)

		for _, s := range c {
			bg.servers[i][s] = nil
			//	bg.serversRPCConnection[i][s] = nil
			bg.serverToSuperLeafMap[s] = i
			bg.superLeafToServers[i] = append(bg.superLeafToServers[i], s)
			if bg.leaders[i] == "" {
				bg.leaders[i] = s
			}
		}
	}

	logger.Debugf("bg server to superleaf map: %v", bg.serverToSuperLeafMap)
	return bg
}

func (bg *ByzantineGroup) GetSuperLeafIdByServer(server string) (int, bool) {
	i, e := bg.serverToSuperLeafMap[server]
	return i, e
}

func (bg *ByzantineGroup) GetAllReplicaAddrBySuperLeafId(slId int) ([]string, bool) {
	addrList, e := bg.superLeafToServers[slId]
	if e {
		return addrList, e
	} else {
		return nil, e
	}
}

func (bg *ByzantineGroup) GetRandomNode(slId int) string {
	i := rand.Intn(len(bg.superLeafToServers[slId]))
	return bg.superLeafToServers[slId][i]
}

func (bg *ByzantineGroup) GetLeader(slId int) string {
	bg.leadersLock.RLock()
	defer bg.leadersLock.RUnlock()
	return bg.leaders[slId]
}

func (bg *ByzantineGroup) GetLeaderByServer(server string) string {
	i, e := bg.serverToSuperLeafMap[server]
	if !e {
		return ""
	}
	bg.leadersLock.RLock()
	defer bg.leadersLock.RUnlock()
	return bg.leaders[i]
}

func (bg *ByzantineGroup) IsLeader(server string) bool {
	return bg.GetLeaderByServer(server) == server
}

func (bg *ByzantineGroup) UpdateLeader(leader string) {
	i, e := bg.serverToSuperLeafMap[leader]
	bg.leadersLock.Lock()
	defer bg.leadersLock.Unlock()
	if e {
		bg.leaders[i] = leader
	}
}

// get one emulater per superleaf
// TODO: for new, just return the monitor ids.
//       It should fetch from Memebership service
func (bg *ByzantineGroup) GetEmulatorList() []string {
	return bg.leaders
}

//func (bg *ByzantineGroup) GetRPCConnection(server string) *grpc.ClientConn {
//	i, _ := bg.GetSuperLeafIdByServer(server)
//	bg.serversRPCConnectionLock.Lock()
//	defer bg.serversRPCConnectionLock.Unlock()
//	if bg.serversRPCConnection[i][server] == nil {
//		logger.Debugf("RPC Connecting %s", server)
//		bg.serversRPCConnection[i][server], _ = grpc.Dial(server, grpc.WithInsecure())
//	}
//	return bg.serversRPCConnection[i][server]
//}

func (bg *ByzantineGroup) GetNewConnection(dstServerAddr string) *Connection {
	//i, e := bg.GetSuperLeafIdByServer(dstServerAddr)
	//if !e {
	//	logger.Fatalf("server %v dose not in the any superleaf", dstServerAddr)
	//}
	logger.Debugf("Create new connection to server %s", dstServerAddr)
	return NewConnection(
		dstServerAddr,
		//bg.superLeafToServers[i],
		queueLen, 10)

}

func (bg *ByzantineGroup) GetConnection(dstServerAddr string) *Connection {
	i, e := bg.GetSuperLeafIdByServer(dstServerAddr)
	if !e {
		logger.Fatalf("server %v dose not in the any superleaf", dstServerAddr)
	}
	bg.serversLock.Lock()
	defer bg.serversLock.Unlock()
	if bg.servers[i][dstServerAddr] == nil {
		logger.Debugf("Connecting %s", dstServerAddr)
		bg.servers[i][dstServerAddr] = NewConnection(
			dstServerAddr,
			//bg.superLeafToServers[i],
			queueLen, 10)
	}
	return bg.servers[i][dstServerAddr]
}

func (bg *ByzantineGroup) GetSuperLeafNum() int {
	return len(bg.leaders)
}
