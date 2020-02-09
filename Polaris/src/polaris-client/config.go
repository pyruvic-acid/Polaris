package main

import (
	"Polaris/src/common"
	"flag"
	"math/rand"
	"os"
	"strconv"
	"time"
)

const (
	INVALID_STR = ""

	// Execution duration, unit: seconds
	FLAG_EXEC_DURATION = "exec.duration"

	// Total number of transactions to execute if duration is not positive
	FLAG_EXEC_TXN_TOTAL_NUM = "exec.txn.num.per.client"

	// Transaction size (byte)
	FLAG_EXEC_TXN_SIZE = "exec.txn.size"

	// The number of transactons sent per second to send
	FLAG_EXEC_TXN_SEND_RATE = "exec.txn.send.rate"

	// Random seed. 0 for a dynamic seed. Default is 0.
	FLAG_RANDOM_SEED = "random.seed"

	DEFAULT_EXEC_DURATION      = "10"
	DEFAULT_EXEC_TXN_TOTAL_NUM = "1000"
	DEFAULT_EXEC_TXN_SIZE      = "32"
	DEFAULT_EXEC_TXN_SEND_RATE = "0"
	DEFAULT_RANDOM_SEED        = "0"
)

// Configuration flags
const (
	CONFIG_DEBUG         = "debug"
	CONFIG_DEBUG_HELPER  = "enables debug mode."
	CONFIG_DEBUG_DEFAULT = false

	CONFIG_CLIENT         = "c"
	CONFIG_CLIENT_HELPER  = "Configuration file for client <REQUIRED>."
	CONFIG_CLIENT_DEFAULT = INVALID_STR

	CONFIG_HOME         = "ch"
	CONFIG_HOME_HELPER  = "A director where the configuration file located <REQUIRED>."
	CONFIG_HOME_DEFAULT = INVALID_STR

	CONFIG_EXEC         = "e"
	CONFIG_EXEC_HELPER  = "Configuration file for execution. <REQUIRED>"
	CONFIG_EXEC_DEFAULT = INVALID_STR

	CONFIG_CLIENT_TOTAL         = "cl"
	CONFIG_CLIENT_TOTAL_HELPER  = "Total number of clients. <REQUIRED>"
	CONFIG_CLIENT_TOTAL_DEFAULT = 0
)

// Config variables
var IsDebug bool
var ClientConfigFile string
var ConfigHome string
var ExecutionConfigFile string
var Duration time.Duration
var RandomSeed int64

// Transaction rate settings
var TxnTotalNum int
var TxnSendRate int
var TxnSize int

var ClientTotalNum int

func ParseArgs() {
	flag.BoolVar(
		&IsDebug,
		CONFIG_DEBUG,
		CONFIG_DEBUG_DEFAULT,
		CONFIG_DEBUG_HELPER,
	)
	flag.StringVar(
		&ClientConfigFile,
		CONFIG_CLIENT,
		CONFIG_CLIENT_DEFAULT,
		CONFIG_CLIENT_HELPER,
	)
	flag.StringVar(
		&ConfigHome,
		CONFIG_HOME,
		CONFIG_HOME_DEFAULT,
		CONFIG_HOME_HELPER,
	)
	flag.StringVar(
		&ExecutionConfigFile,
		CONFIG_EXEC,
		CONFIG_EXEC_DEFAULT,
		CONFIG_EXEC_HELPER,
	)
	flag.IntVar(
		&ClientTotalNum,
		CONFIG_CLIENT_TOTAL,
		CONFIG_CLIENT_TOTAL_DEFAULT,
		CONFIG_CLIENT_TOTAL_HELPER,
	)

	flag.Parse()

	if !validStr(ClientConfigFile) ||
		!validStr(ExecutionConfigFile) ||
		!validStr(ConfigHome) ||
		ClientTotalNum == 0 {
		flag.Usage()
		os.Exit(-1)
	}
}

func validStr(str string) bool {
	if str == INVALID_STR {
		return false
	}
	return true
}

func LoadExecutionConfig() {

	execConfig := common.LoadProperties(ExecutionConfigFile)

	// Execution duration in seconds
	configVal := common.GetProperty(
		execConfig,
		FLAG_EXEC_DURATION,
		DEFAULT_EXEC_DURATION)
	if duration, err := strconv.ParseInt(configVal, 10, 64); err != nil {
		logger.Fatalf("Invalid duration value = %s", configVal)
	} else {
		Duration = time.Duration(duration * int64(time.Second))
	}

	var parseErr error

	configVal = common.GetProperty(
		execConfig,
		FLAG_EXEC_TXN_TOTAL_NUM,
		DEFAULT_EXEC_TXN_TOTAL_NUM,
	)
	if TxnTotalNum, parseErr = strconv.Atoi(configVal); parseErr != nil {
		logger.Fatalf("Invalid total number of transactions to execute. value = %s", configVal)
	}

	configVal = common.GetProperty(
		execConfig,
		FLAG_EXEC_TXN_SIZE,
		DEFAULT_EXEC_TXN_SIZE,
	)
	if TxnSize, parseErr = strconv.Atoi(configVal); parseErr != nil {
		logger.Fatalf("Invalid transaction size. value = %s", configVal)
	}

	configVal = common.GetProperty(
		execConfig,
		FLAG_EXEC_TXN_SEND_RATE,
		DEFAULT_EXEC_TXN_SEND_RATE,
	)
	if TxnSendRate, parseErr = strconv.Atoi(configVal); parseErr != nil {
		logger.Fatalf("Invalid transaction send rate (per client), value = %s", configVal)
	}

	// Random seed
	configVal = common.GetProperty(
		execConfig,
		FLAG_RANDOM_SEED,
		DEFAULT_RANDOM_SEED)
	if RandomSeed, parseErr = strconv.ParseInt(configVal, 10, 64); parseErr != nil {
		logger.Fatalf("Invalid randomSeed value = %s", configVal)
	} else {
		if RandomSeed == 0 {
			RandomSeed = int64(time.Now().Nanosecond())
		}
		rand.Seed(RandomSeed)
	}

	logger.Debug("Execution Configurations:", execConfig)
	logger.Debug("Debug mode =", IsDebug)
	logger.Debug("Client config file =", ClientConfigFile)
	logger.Debug("Config home =", ConfigHome)
	logger.Debug("Total number of client =", ClientTotalNum)
	logger.Debug("Execution config file =", ExecutionConfigFile)
	logger.Debug("Running duration =", Duration)
	logger.Debug("Total number of transactions =", TxnTotalNum)
	logger.Debug("Send transaction rate =", TxnSendRate)
	logger.Debug("Random seed =", RandomSeed)
}
