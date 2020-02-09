package common

import (
	"os"

	"github.com/op/go-logging"
)

var loggerFormat = logging.MustStringFormatter(
	//`%{color}%{time:15:04} %{module} %{shortfunc} > %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	`%{color}%{time:15:04:05.000} %{module} %{shortfunc} > %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

func ConfigLogger(isDebug bool) {
	// Logger settings
	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logBackendFormat := logging.NewBackendFormatter(logBackend, loggerFormat)
	logBackendLevel := logging.AddModuleLevel(logBackendFormat)
	logBackendLevel.SetLevel(logging.ERROR, "")
	logBackendLevel.SetLevel(logging.CRITICAL, "")
	logBackendLevel.SetLevel(logging.INFO, "")
	logBackendLevel.SetLevel(logging.WARNING, "")
	if isDebug {
		logBackendLevel.SetLevel(logging.DEBUG, "")
	}
	logging.SetBackend(logBackendLevel)
}

func AssertTrue(expr bool, msg interface{}, logger *logging.Logger) {
	if !expr {
		logger.Fatal(msg)
	}
}

func AssertNil(expr interface{}, msg interface{}, logger *logging.Logger) {
	if expr != nil {
		logger.Debug(expr)
		logger.Fatal(msg)
	}
}
