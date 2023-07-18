// androidqf - Android Quick Forensics
// Copyright (c) 2021-2023 Claudio Guarnieri.
// Use of this software is governed by the MVT License 1.1 that can be found at
//   https://license.mvt.re/1.1/

package log

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/i582/cfmt/cmd/cfmt"
)

type LEVEL uint8

const (
	DEBUG LEVEL = iota + 1
	INFO
	WARNING
	ERROR
	CRITICAL
	FATAL
)

type Logger struct {
	LogLevel     LEVEL
	FileLogLevel LEVEL
	fd           *os.File
	fileName     string
	Color        bool
}

var (
	log  *Logger
	once sync.Once
)

// New returns plain Logger instance
func New() *Logger {
	l := &Logger{
		LogLevel:     INFO,
		FileLogLevel: DEBUG,
		fd:           nil,
		fileName:     "",
		Color:        true,
	}
	return l
}

func init() {
	Get()
}

// Get returns singleton Logger instance
func Get() *Logger {
	once.Do(func() {
		log = New()
	})
	return log
}

func (log *Logger) out(level LEVEL, format string, v ...any) {
	// Start with printing in the console
	if level >= log.LogLevel {
		var msg string
		if format == "" {
			msg = fmt.Sprint(v...)
		} else {
			msg = fmt.Sprintf(format, v...)
		}
		// for debug message,
		if level == DEBUG {
			msg = fmt.Sprintf("DEBUG: %s", msg)
		}
		// Make sure to trim end of line
		msg = strings.TrimSuffix(msg, "\n")
		if log.Color {
			if level > INFO {
				cfmt.Printf("{{%s}}::red|bold\n", msg)
			} else {
				fmt.Println(msg)
			}
		} else {
			fmt.Println(msg)
		}
	}
	// Print in the file if any
	if log.fd != nil {
		var msg string
		if level >= log.FileLogLevel {
			if format == "" {
				msg = fmt.Sprint(v...)
			} else {
				msg = fmt.Sprintf(format, v...)
			}
			fmt.Fprintf(log.fd, "%s [%s] %s\n", time.Now().Format(time.RFC3339), level.String(), msg)
		}
	}
}

func (l LEVEL) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case CRITICAL:
		return "CRITICAL"
	case FATAL:
		return "FATAL"
	}
	return ""
}

func SetLogLevel(level LEVEL) {
	log.LogLevel = level
}

func Coloring(enable bool) {
	log.Color = enable
}

func EnableFileLog(level LEVEL, filePath string) error {
	if filePath == "" {
		return errors.New("invalid file path")
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	log.fd = file
	log.fileName = filePath
	return nil
}

func DisableFileLog() {
	log.fd.Close()
	log.fd = nil
	log.fileName = ""
}

func Debug(v ...any) {
	log.out(DEBUG, "", v...)
}

func Debugf(format string, v ...any) {
	log.out(DEBUG, format, v...)
}

func Info(v ...any) {
	log.out(INFO, "", v...)
}

func Infof(format string, v ...any) {
	log.out(INFO, format, v...)
}

func Warning(v ...any) {
	log.out(WARNING, "", v...)
}

func Warningf(format string, v ...any) {
	log.out(WARNING, format, v...)
}

func Error(v ...any) {
	log.out(ERROR, "", v...)
}

func Errorf(format string, v ...any) {
	log.out(ERROR, format, v...)
}

func ErrorExc(desc string, err error) {
	log.out(ERROR, "ERROR: %s: %s\n", desc, err.Error())
}

func Critical(v ...any) {
	log.out(CRITICAL, "", v...)
}

func Criticalf(format string, v ...any) {
	log.out(CRITICAL, format, v...)
}

func Fatal(v ...any) {
	log.out(FATAL, "", v...)
	os.Exit(1)
}

func Fatalf(format string, v ...any) {
	log.out(FATAL, format, v...)
	os.Exit(1)
}

func FatalExc(desc string, err error) {
	log.out(FATAL, "FATAL: %s: %s\n", desc, err.Error())
	os.Exit(1)
}
