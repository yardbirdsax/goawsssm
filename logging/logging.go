package logging

import "context"

const (
	LOGGER_CONTEXT_KEY string = "logger"
)

type Logger interface {
	Debugf(format string, args ...interface{})
	Debug(args ...interface{})
	Infof(format string, args ...interface{})
	Info(args ...interface{})
	Warnf(format string, args ...interface{})
	Warn(args ...interface{})
	Errorf(format string, args ...interface{})
	Error(args ...interface{})
}

func GetLogger(ctx context.Context) (logger Logger) {
	logContext := ctx.Value(LOGGER_CONTEXT_KEY)
	if logContext != nil {
		logger = logContext.(Logger)
	} else {
		logger = nil
	}
	return
}

func Debugf(logger Logger, format string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(format, args)
	}
}

func Infof(logger Logger, format string, args ...interface{}) {
	if logger != nil {
		logger.Infof(format, args)
	}
}

func Warnf(logger Logger, format string, args ...interface{}) {
	if logger != nil {
		logger.Warnf(format, args)
	}
}

func Errorf(logger Logger, format string, args ...interface{}) {
	if logger != nil {
		logger.Errorf(format, args)
	}
}
