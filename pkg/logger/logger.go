/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
//
// Log implementation for all microservices in the project.
// Log functions can be called through the convenience interfaces
// logger.Debugf(), logger.Errorf(), logger.Panicf()
//
// Deliberately reduces the interface to only Debugf, Errorf and Panicf.
// The other log levels are discouraged (see fdc Software Engineering Standards
// for details)
package logger

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
)

var (
	serviceName   string
	defaultLogger *logrus.Logger
)

func init() {

	SetLogLevel(os.Getenv("LOG_LEVEL"))
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "<unknwon hostname>"
	}
	SetServiceName(hostname)

	logger := logrus.StandardLogger()
	logrus.SetFormatter(&logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyMsg: "message",
		},
	})
	logrus.AddHook(&severityHook{})
	logger.AddHook(&stacktraceHook{})

	SetDefaultLogger(logger)
}

// SetDefaultLogger overrides the logger that is used in the convenience interface
// that can be accessed from everywhere. Changing the default logger should be
// done very early in the programs main function.
func SetDefaultLogger(l *logrus.Logger) {
	defaultLogger = l

	// independent from our log level we are not interested in log messages but errors for 3rd party libraries
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, l.Writer()))
}

// SetServiceName allows overwriting default service name 'hostname'
func SetServiceName(newServiceName string) {
	serviceName = newServiceName
}

// SetLogLevel allows overwriting default log level 'Error'
func SetLogLevel(logLevel string) {
	logrus.SetLevel(getLogrusLogLevel(logLevel))
}

func getLogrusLogLevel(logLevel string) logrus.Level {
	switch logLevel {
	case "": // not set
		return logrus.ErrorLevel
	case "panic":
		return logrus.PanicLevel
	case "fatal":
		return logrus.FatalLevel
	case "error":
		return logrus.ErrorLevel
	case "warn":
		return logrus.WarnLevel
	case "info":
		return logrus.InfoLevel
	case "debug":
		return logrus.DebugLevel
	case "trace":
		return logrus.TraceLevel
	}

	panic(fmt.Sprintf("LOG_LEVEL %s is not known", logLevel))
}

//convenience interface

func Panicf(format string, args ...interface{}) {
	defaultLogger.Panicf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

//structured logging support

func WithError(err error) *logrus.Entry {
	return defaultLogger.WithError(err)
}

func WithField(key string, value interface{}) *logrus.Entry {
	return defaultLogger.WithField(key, value)
}

type stacktraceHook struct {
}

func (*stacktraceHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (*stacktraceHook) Fire(e *logrus.Entry) error {
	if e != nil {
		if err, ok := e.Data[logrus.ErrorKey]; ok {
			e.Data[logrus.ErrorKey] = fmt.Sprintf("%+v", err)
		}
	}
	return nil
}

var (
	logrusToGCE = map[logrus.Level]string{
		logrus.TraceLevel: "NOTICE",
		logrus.DebugLevel: "DEBUG",
		logrus.InfoLevel:  "INFO",
		logrus.WarnLevel:  "WARNING",
		logrus.ErrorLevel: "ERROR",
		logrus.FatalLevel: "CRITICAL",
		logrus.PanicLevel: "EMERGENCY",
	}
)

type severityHook struct {
}

// Levels -> tell logrus which levels are interesting for us: All
func (*severityHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire -> add field "severity" to the log-line
func (*severityHook) Fire(e *logrus.Entry) error {
	if e != nil {
		e.Data["severity"] = logrusToGCE[e.Level]
	}
	return nil
}

type key struct{}

func FromContext(ctx context.Context) *logrus.Logger {
	logger := ctx.Value(key{})
	if logger == nil {
		return defaultLogger
	} else {
		return logger.(*logrus.Logger)
	}
}

func WithLogger(ctx context.Context, logger *logrus.Logger) context.Context {
	return context.WithValue(ctx, key{}, logger)
}
