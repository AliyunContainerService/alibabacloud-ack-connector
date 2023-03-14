package logging

import (
	"fmt"
	"github.com/alibaba/alibabacloud-ack-connector/pkg/utils"
	"runtime"

	"github.com/sirupsen/logrus"
)

func NewLogger(level int) *logrus.Logger {
	logger := logrus.New()
	logger.SetReportCaller(true)
	logger.Formatter = &logrus.TextFormatter{
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := utils.TrimStrAndPre(f.File, "/src/github.com/alibaba/alibabacloud-ack-connector")
			return "", fmt.Sprintf("%s:%d", filename, f.Line)
		},
	}

	switch level {
	case -1:
		logger.SetLevel(logrus.TraceLevel)
	case 0:
		logger.SetLevel(logrus.DebugLevel)
	case 1:
		logger.SetLevel(logrus.InfoLevel)
	case 2:
		logger.SetLevel(logrus.WarnLevel)
	default:
		logger.SetLevel(logrus.ErrorLevel)
	}
	return logger
}
