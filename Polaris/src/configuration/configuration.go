package configuration

import (
	"Polaris/src/common"
	"bufio"

	//"crypto/rsa"
	"os"
	"strconv"
	"strings"

	"github.com/op/go-logging"
)

var logger = logging.MustGetLogger("configuration")

type Configuration struct {
	id string
	//	configs              map[string]string
	configHome           string
	systemConfigFileName string
	//keys                 *KeysConfig
	//keyPath              string

	f int32
	// request timeout.
	// TODO: no view change for now, use this for view change
	requestTimeout int32 // second
	// cycle timeout. When expire before it collects enough transactions
	// it will start to order the buffered transactions
	bftCycleTimeout int32 // millisecond
	bftBatchSize    int32

	clientRequestCycleTimeout int32

	// max number of request buffered per cycle
	maxBatchSize int32
	// max number of connection events
	maxConnectionEventQueueSize int32
	// if it is true, enable the debug mode
	debug bool
	// if use signature
	useSignature bool

	isHeaderBodySeparate bool

	maxPipelineDepth int32
}

func NewConfiguration(id string, configHome string) *Configuration {
	c := &Configuration{
		id:                   id,
		configHome:           configHome,
		systemConfigFileName: common.SYSCFGFILE,
		//keyPath:              common.SYSKEYPATH,
	}

	configs := LoadConfig(c.configHome, c.systemConfigFileName)
	c.processConfigs(configs)

	//if c.useSignature {
	//	c.keys = NewKeysConfig(c.configHome, c.keyPath, c.id)
	//}

	return c
}

func (c *Configuration) GetF() int32 {
	return c.f
}

func (c *Configuration) GetMaxConnectionEventQueueSize() int32 {
	return c.maxConnectionEventQueueSize
}

func (c *Configuration) GetBFTCycleTimeout() int32 {
	return c.bftCycleTimeout
}

func (c *Configuration) GetClientRequestCycleTimeout() int32 {
	return c.clientRequestCycleTimeout
}

//func (c *Configuration) GetPrivateKey() *rsa.PrivateKey {
//	return c.keys.GetPrivateKey()
//}
//
//func (c *Configuration) GetPublicKeyById(id string) *rsa.PublicKey {
//	return c.keys.GetPublicKeyById(id)
//}
//
//func (c *Configuration) GetClientPublicKeyById(id string) *rsa.PublicKey {
//	return c.keys.GetClientPublicKeyById(id)
//}

func (c *Configuration) GetServerId() string {
	return c.id
}

func (c *Configuration) SetServerId(id string) {
	c.id = id
}

func (c *Configuration) GetRequestTimeout() int32 {
	return c.requestTimeout
}

func (c *Configuration) GetMaxBatchSize() int32 {
	return c.maxBatchSize
}

func (c *Configuration) GetBFTBatchSize() int32 {
	return c.bftBatchSize
}

func (c *Configuration) GetMaxPipelineDepth() int32 {
	return c.maxPipelineDepth
}

func (c *Configuration) IsDebug() bool {
	return c.debug
}

func (c *Configuration) UseSignature() bool {
	return c.useSignature
}

func (c *Configuration) IsHeaderBodySeparate() bool {
	return c.isHeaderBodySeparate
}

func (c *Configuration) processConfigs(configs map[string]string) {

	fStr, ok := configs["failures"]
	common.AssertTrue(ok, "should privde f", logger)
	f, err := strconv.Atoi(fStr)
	common.AssertNil(err, "the failures should be an integer", logger)
	c.f = int32(f)

	// request timeout
	if timeoutStr, ok := configs["request.timeout"]; ok {
		timeout, err := strconv.Atoi(timeoutStr)
		common.AssertNil(err, "The timeout should be an integer", logger)
		c.requestTimeout = int32(timeout)
	} else {
		// default 1 sec
		c.requestTimeout = 1
	}

	if cycleTimeoutStr, ok := configs["bft.cycle.timeout"]; ok {
		if cycleTimeout, err := strconv.Atoi(cycleTimeoutStr); err == nil {
			c.bftCycleTimeout = int32(cycleTimeout)
		} else {
			logger.Fatal("The cycle timeout should be int")
		}
	} else {
		// default 100 ms
		c.bftCycleTimeout = 100
	}

	if cycleTimeoutStr, ok := configs["client.request.cycle.timeout"]; ok {
		if cycleTimeout, err := strconv.Atoi(cycleTimeoutStr); err == nil {
			c.clientRequestCycleTimeout = int32(cycleTimeout)
		} else {
			logger.Fatal("The cycle timeout should be int")
		}
	} else {
		// default 100 ms
		c.clientRequestCycleTimeout = 100
	}

	// batch size
	if batchStr, ok := configs["max.batch"]; ok {
		batch, err := strconv.Atoi(batchStr)
		common.AssertNil(err, "the max batch size should be an integer", logger)
		c.maxBatchSize = int32(batch)
	} else {
		c.maxBatchSize = 100
	}

	// pbft batch size
	if batchStr, ok := configs["max.bft.batch"]; ok {
		batch, err := strconv.Atoi(batchStr)
		common.AssertNil(err, "the max batch size should be an integer", logger)
		c.bftBatchSize = int32(batch)
	} else {
		c.bftBatchSize = 4
	}

	if connectionEventQueueStr, ok := configs["max.connection.event.queue.size"]; ok {
		connectionEventQueue, err := strconv.Atoi(connectionEventQueueStr)
		common.AssertNil(err, "the max connection event queue size should be an integer", logger)
		c.maxConnectionEventQueueSize = int32(connectionEventQueue)
	} else {
		c.maxConnectionEventQueueSize = 64
	}

	if pipelineDepthStr, ok := configs["max.pipeline.depth"]; ok {
		pipelineDepth, err := strconv.Atoi(pipelineDepthStr)
		common.AssertNil(err, "the max pipeline depth should be an integer", logger)
		c.maxPipelineDepth = int32(pipelineDepth)
	} else {
		c.maxPipelineDepth = 16
	}

	if debugStr, ok := configs["debug"]; ok {
		debug, err := strconv.ParseBool(debugStr)
		common.AssertNil(err, "The debug should be true/false", logger)
		c.debug = debug
	} else {
		c.debug = false
	}

	if signatureStr, ok := configs["signature"]; ok {
		signature, err := strconv.ParseBool(signatureStr)
		common.AssertNil(err, "The signature should be true/false", logger)
		c.useSignature = signature
	} else {
		c.useSignature = true
	}

	if isHeaderBodySeparateStr, ok := configs["header.body.separate"]; ok {
		isHeaderBodySeparate, err := strconv.ParseBool(isHeaderBodySeparateStr)
		common.AssertNil(err, "The isHeaderBodySeparate should be true/false", logger)
		c.isHeaderBodySeparate = isHeaderBodySeparate
	} else {
		c.isHeaderBodySeparate = true
	}

}

func LoadConfig(configHome string, configFileName string) map[string]string {
	if configHome == "" {
		configHome = "config"
	}

	if configFileName == "" {
		configFileName = "system.config"
	}

	path := configHome + "/" + configFileName

	file, err := os.Open(path)

	common.AssertNil(err, "Cannot open the file", logger)

	defer file.Close()

	scanner := bufio.NewScanner(file)

	configs := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") && line != "" && line != "\n" {
			items := strings.SplitN(line, "=", 2)
			if items != nil {
				configs[strings.Trim(items[0], " ")] = strings.Trim(items[1], " ")
			} else {
				logger.Infof("Parsing error, Skipping line")
			}
		}
	}

	err = scanner.Err()
	common.AssertNil(err, "Scanner throws an exception", logger)

	return configs
}
