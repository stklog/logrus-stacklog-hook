package stklog

import (
	"bytes"
	"errors"
	"runtime"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

// hook struct implementing Logrus Hook interface (https://github.com/sirupsen/logrus/blob/master/hooks.go)
// containing logLevels defined by SetLevel
type StklogHook struct {
	logLevels []logrus.Level
}

// Factory to create a new Hook
// will initiate a goroutine to bufferise and send logs to stklog.io
func NewStklogHook(apiKey string) *StklogHook {
	start(apiKey)
	return &StklogHook{}
}

// Standard method called by logrus when a log is written
// we normalize the log and send it to a channel to be bufferised and sent later
func (hook *StklogHook) Fire(entry *logrus.Entry) error {
	message := bytes.TrimSpace([]byte(entry.Message))
	requestID, ok := mapping[getGID()]
	if ok == false {
		return errors.New(STACK_NOT_FOUND)
	}
	file, line := getCaller(1)
	logMessage := &LogMessage{
		// logrus levels are lower than syslog by 2
		Level:     int32(entry.Level) + 2,
		Extra:     entry.Data,
		Message:   string(message),
		Timestamp: entry.Time.Format(time.RFC3339),
		File:      file,
		Line:      line,
		RequestID: requestID,
	}
	chanBuffer <- iMessage(logMessage)
	return nil
}

// getCaller returns the filename and the line info of a function
// further down in the call stack.  Passing 0 in as callDepth would
// return info on the function calling getCallerIgnoringLog, 1 the
// parent function, and so on.  Any suffixes passed to getCaller are
// path fragments like "/pkg/log/log.go", and functions in the call
// stack from that file are ignored.
// from https://github.com/gemnasium/logrus-graylog-hook/blob/master/graylog_hook.go
func getCaller(callDepth int, suffixesToIgnore ...string) (file string, line int) {
	// bump by 1 to ignore the getCaller (this) stackframe
	callDepth++
outer:
	for {
		var ok bool
		_, file, line, ok = runtime.Caller(callDepth)
		if !ok {
			file = "???"
			line = 0
			break
		}

		for _, s := range suffixesToIgnore {
			if strings.HasSuffix(file, s) {
				callDepth++
				continue outer
			}
		}
		break
	}
	return
}

// from https://github.com/gemnasium/logrus-graylog-hook/blob/master/graylog_hook.go
func getCallerIgnoringLogMulti(callDepth int) (string, int) {
	// the +1 is to ignore this (getCallerIgnoringLogMulti) frame
	return getCaller(callDepth+1, "logrus/hooks.go", "logrus/entry.go", "logrus/logger.go", "logrus/exported.go", "asm_amd64.s")
}

// Standard method to implement for the hook interface, telling logrus which levels it needs to call us for
func (hook *StklogHook) Levels() []logrus.Level {
	if len(hook.logLevels) == 0 {
		return logrus.AllLevels
	}
	return hook.logLevels
}

// Flush allow you to send last things stuck in the before when you want to quit, since we use a ticker to send logs / stacks to the platform.
func (hook *StklogHook) Flush() {
	flusher <- true
	<-flusher
}

// Custom method for user to define from which level he/she wants to logs to Stklog
func (hook *StklogHook) SetLevel(level logrus.Level) {
	for _, element := range logrus.AllLevels {
		if int32(element) <= int32(level) {
			hook.logLevels = append(hook.logLevels, element)
		}
	}
}
