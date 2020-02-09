package main

import (
	"Polaris/src/client"
	"Polaris/src/common"
	"Polaris/src/util"

	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/op/go-logging"
)

type txnStat struct {
	Header      []byte
	Msg         []byte
	SendTime    time.Time
	ReceiveTime time.Time
	SenderId    string
	CycleId     int
}

var logger = logging.MustGetLogger("polaris-client")
var sendTxn = make(map[int]time.Time)
var receivedTxn = make([]*txnStat, 0, 100000)
var bodyHeaderMap = make(map[int]int)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func main() {
	ParseArgs()

	common.ConfigLogger(IsDebug)

	// Loads benchmark configurations
	LoadExecutionConfig()

	c := client.NewClient(ClientConfigFile, ConfigHome)

	//go runTransaction(c)

	var execMode ExecutionMode
	if TxnTotalNum > 0 {
		execMode = NewFixedTxnMode(TxnTotalNum, ClientTotalNum, TxnSendRate)
	} else {
		execMode = NewDurationMode(Duration, TxnSendRate)
	}

	go execMode.execute(c)

	//waitForCycleResult(c)
	execMode.getCycleResult(c)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
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

func printLog(c *client.Client, expStartTime time.Time) {
	orderFileName := fmt.Sprintf("order_%v.log", c.ID)
	orderF, err := os.Create(orderFileName)
	check(err)
	defer orderF.Close()

	latencyFileName := fmt.Sprintf("latency_%v.log", c.ID)
	latencyF, err := os.Create(latencyFileName)
	check(err)
	defer latencyF.Close()

	// write experiment config
	config := fmt.Sprintf("# msgSize: %v, messages per client: %v, totalClient: %v\n",
		TxnSize, TxnTotalNum, ClientTotalNum)
	headerL := "# txnContent, latency(ns), start time, receive time, cycle Id\n"
	headerO := "# txnContent, cycleId\n"
	_, err = latencyF.WriteString(config)
	_, err = latencyF.WriteString(headerL)
	_, err = orderF.WriteString(config)
	_, err = orderF.WriteString(headerO)
	check(err)

	for _, t := range receivedTxn {
		line := ""
		receiveTime := int64(t.ReceiveTime.Sub(expStartTime).Nanoseconds() / 1000)
		h := hash(t.Header)
		if len(t.Header) == 0 {
			h = bodyHeaderMap[hash(t.Msg)]
		}
		if st, exist := sendTxn[h]; exist && t.SenderId == c.ID {
			latency := int64(t.ReceiveTime.Sub(st).Nanoseconds() / 1000)  // us
			startTime := int64(st.Sub(expStartTime).Nanoseconds() / 1000) // us
			line = fmt.Sprintf("%x %d %d %d %d\n", h, latency, startTime, receiveTime, t.CycleId)
			_, err = latencyF.WriteString(line)
			check(err)
		}
		line = fmt.Sprintf("%x %d\n", hash(t.Msg), t.CycleId)
		_, err = orderF.WriteString(line)
		check(err)
	}
	orderF.Sync()
	latencyF.Sync()
}

func GenerateRandomBytes(n int) []byte {
	//b := make([]byte, n)
	//for i := uint64(0); i < n; i++ {
	//	b[i] = 'a'
	//}
	//	_, err := rand.Read(b)
	//	// Note that err == nil only if we read len(b) bytes.
	//	if err != nil {
	//		return nil, err
	//	}
	//return b, nil

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return []byte(string(b))

}

type ExecutionMode interface {
	execute(client *client.Client)
	getCycleResult(client *client.Client)
}

type DurationMode struct {
	duration       time.Duration
	durationPerTxn time.Duration
	txnPerSecond   int
	startTime      time.Time
	finish         chan bool
}

func NewDurationMode(duration time.Duration, sendRate int) *DurationMode {
	d := &DurationMode{
		duration:       duration,
		durationPerTxn: time.Duration(time.Second.Nanoseconds() / int64(sendRate)),
		finish:         make(chan bool, 1),
		txnPerSecond:   sendRate,
	}

	return d
}

func (d *DurationMode) execute(client *client.Client) {
	logger.Debugf("duration mode start: %v", d.duration)
	totalCount := 0

	txnEstimate := int(((d.duration / time.Second).Seconds() + 2.0) * float64(d.txnPerSecond)) // +1 for extra buffer
	rgTxn := make([][]byte, txnEstimate)
	rgTxnHeader := make([]string, txnEstimate)
	rgHashOfHeader := make([]int, txnEstimate)

	// Pre-calculate
	calcStart := time.Now()
	for i := 1; i < txnEstimate; i++ {
		rgTxn[i] = GenerateRandomBytes(TxnSize)
		rgTxnHeader[i] = util.ComputeTxnHash(rgTxn[i])
		rgHashOfHeader[i] = hash([]byte(rgTxnHeader[i]))
	}

	// Create collision! Test the collision resolution
	rgTxn[0] = rgTxn[1]
	rgTxnHeader[0] = rgTxnHeader[1]
	rgHashOfHeader[0] = rgHashOfHeader[1]

	txnPtr := 0
	// Exclude the time for pre-calculation
	d.startTime = time.Now()
	execTime := time.Duration(0)
	for execTime < d.duration {
		var txnData []byte
		var header string
		var h int

		var tmphash int = 0
		for fCollision := true; fCollision; txnPtr++ {
			if txnPtr < txnEstimate {
				txnData = rgTxn[txnPtr]
				header = rgTxnHeader[txnPtr]
				h = rgHashOfHeader[txnPtr]
			} else {
				txnData = GenerateRandomBytes(TxnSize)
				header = util.ComputeTxnHash(txnData)
				h = hash([]byte(header))
			}

			fCollision = false
			tmphash = hash(txnData)
			if _, exist := sendTxn[h]; exist {
				logger.Warningf("Hash collision on txn header %v", header)
				fCollision = true
			}
			if _, exist := bodyHeaderMap[tmphash]; exist {
				logger.Warningf("Hash collision on txn body (hash = %d)", tmphash)
				fCollision = true
			}
		}

		bodyHeaderMap[tmphash] = h

		client.SendTxn(txnData, []byte(header))
		totalCount++

		sendTxn[h] = time.Now()

		if totalCount%1000 == 0 {
			logger.Warningf("%d txn sent", totalCount)
		}

		execTime = time.Since(d.startTime)
		execTimeInSeconds := execTime.Seconds()
		stdTimeInSeconds := float64(totalCount) / float64(d.txnPerSecond)
		sleepDurationInSeconds := stdTimeInSeconds - execTimeInSeconds
		if sleepDurationInSeconds > 0.0001 { // 100us
			time.Sleep(time.Duration(int(sleepDurationInSeconds * float64(time.Second.Nanoseconds()))))
		}

	}

	logger.Debugf("finish sending push finish")
	logger.Warningf("Total number of txn sent: %d; took %v seconds; avg sending rate %v", totalCount, d.duration.Seconds(), float64(totalCount)/d.duration.Seconds())
	logger.Warningf("Pre-calculation time: %v", d.startTime.Sub(calcStart))
	d.finish <- true
	logger.Debugf("finish sending")
}

func (d *DurationMode) getCycleResult(client *client.Client) {
	for {
		select {
		case <-d.finish:
			logger.Debugf("execution time expired")
			printLog(client, d.startTime)
			logger.Warning("Finish printing logs")
			return
		default:
			result := client.GetNextCycle()
			if result == nil {
				<-d.finish
				printLog(client, d.startTime)
				logger.Warning("Finish printing logs")
				return
			}
			ts := time.Now()
			txnList := make([]*txnStat, len(result))
			count := 0
			for _, t := range result {
				txnList[count] = &txnStat{
					Header:      t.Header,
					Msg:         t.Body,
					ReceiveTime: ts,
					SenderId:    t.SenderId,
					CycleId:     t.CycleId,
				}
				count++

			}
			receivedTxn = append(receivedTxn, txnList...)
			logger.Debugf("received txn: %v", len(receivedTxn))
		}
	}
}

// send fix number of txn at a rate
type FixedTxnMode struct {
	txnPerClient   int
	totalTxn       int
	startTime      time.Time
	durationPerTxn time.Duration
	finish         chan bool
}

func NewFixedTxnMode(txnPerClient int, totalClient int, sendRate int) *FixedTxnMode {
	f := &FixedTxnMode{
		txnPerClient:   txnPerClient,
		totalTxn:       txnPerClient * totalClient,
		durationPerTxn: time.Duration(time.Second.Nanoseconds() / int64(sendRate)),
		finish:         make(chan bool, 1),
	}

	return f
}

func (f *FixedTxnMode) execute(client *client.Client) {
	f.startTime = time.Now()
	txnSent := 0
	for txnSent < f.txnPerClient {
		txnData := GenerateRandomBytes(TxnSize)
		header := util.ComputeTxnHash(txnData)
		h := hash([]byte(header))
		if _, exist := sendTxn[h]; exist {
			logger.Warningf("already exit %v", header)
		}
		if _, exist := bodyHeaderMap[hash(txnData)]; exist {
			logger.Warningf("body ready exist %v", txnData)
		}
		bodyHeaderMap[hash(txnData)] = h
		sendTxn[h] = time.Now()
		client.SendTxn(txnData, []byte(header))

		time.Sleep(f.durationPerTxn)
		txnSent++
		logger.Debugf("txn sent: %v", txnSent)
	}

	f.finish <- true
	logger.Debugf("finish sending")
}

func (f *FixedTxnMode) getCycleResult(client *client.Client) {
	for {
		result := client.GetNextCycle()
		if result == nil {
			<-f.finish
			printLog(client, f.startTime)
			logger.Warning("Finish printing logs")
			return
		}
		ts := time.Now()
		txnList := make([]*txnStat, len(result))
		count := 0
		for _, t := range result {
			txnList[count] = &txnStat{
				Header:      t.Header,
				Msg:         t.Body,
				ReceiveTime: ts,
				SenderId:    t.SenderId,
				CycleId:     t.CycleId,
			}
			count++
		}
		receivedTxn = append(receivedTxn, txnList...)

		if len(receivedTxn) == f.totalTxn {
			logger.Debugf("total txn %v is reached", f.totalTxn)
			printLog(client, f.startTime)
			break
		}
		logger.Debugf("total txn: %v; received txn: %v", f.totalTxn, len(receivedTxn))
	}
}
