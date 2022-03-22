package ylog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type MyFormatter struct{}

var levelList = []string{
	"PANIC",
	"FATAL",
	"ERROR",
	"WARN",
	"INFO",
	"DEBUG",
	"TRACE",
}

func (mf *MyFormatter) Format(entry *log.Entry) ([]byte, error) {
	level := levelList[int(entry.Level)]
	strList := strings.Split(entry.Caller.File, "/")
	fileName := strList[len(strList)-1]
	msg := fmt.Sprintf("%s %s %s:%d %s\n",
		entry.Time.Format("2006-01-02 15:04:05"), level,
		fileName, entry.Caller.Line, entry.Message)
	return []byte(msg), nil
}

func Init() {
	// Redirect log messages to ~/yb_migrate.log .
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialise logging: get user home dir: %s", err))
	}
	logFileName := filepath.Join(homeDir, "yb_migrate.log")
	f, err := os.OpenFile(logFileName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialise logging: open log file %q: %s", logFileName, err))
	}
	log.SetOutput(f)

	// 2022-03-22 07:07:20 INFO ylog.go:69 Logging initialised.
	log.SetReportCaller(true)
	log.SetFormatter(&MyFormatter{})

	Infof(nil, "Logging initialised.")
}

func Info(ctx context.Context, msg string) {
	log.Info(msg)
}

func Infof(ctx context.Context, fstr string, args ...interface{}) {
	log.Infof(fstr, args...)
}

func Error(ctx context.Context, msg string) {
	log.Error(msg)
}

func Errorf(ctx context.Context, fstr string, args ...interface{}) {
	log.Errorf(fstr, args...)
}

func Warn(ctx context.Context, msg string) {
	log.Warn(msg)
}

func Warnf(ctx context.Context, fstr string, args ...interface{}) {
	log.Warnf(fstr, args...)
}

func Debug(ctx context.Context, msg string) {
	log.Debug(msg)
}

func Debugf(ctx context.Context, fstr string, args ...interface{}) {
	log.Debugf(fstr, args...)
}

func Fatal(ctx context.Context, msg string) {
	log.Fatal(msg)
}

func Fatalf(ctx context.Context, fstr string, args ...interface{}) {
	log.Fatalf(fstr, args...)
}
