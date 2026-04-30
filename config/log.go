package config

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path/filepath"
)

const (
	defaultNodeHealthLogPath          = "data/logs/node_health.log"
	defaultNodeStatusLogPath          = "data/logs/node_status.log"
	defaultTaskAssignmentLogPath      = "data/logs/task_assignment.log"
	defaultTaskValidationGroupLogPath = "data/logs/task_validation_group.log"
	defaultAdminLogPath               = "data/logs/admin.log"
)

var nodeHealthLogger *logrus.Logger
var nodeStatusLogger *logrus.Logger
var taskAssignmentLogger *logrus.Logger
var taskValidationGroupLogger *logrus.Logger
var adminLogger *logrus.Logger

func InitLog(appConfig *AppConfig) error {

	println("Initializing logger...")

	logrus.SetFormatter(&logrus.TextFormatter{})

	switch appConfig.Log.Output {
	case "", "stderr":
		logrus.SetOutput(os.Stderr)
	case "stdout":
		logrus.SetOutput(os.Stdout)
	default:
		logWriter := newLogWriter(appConfig.Log.Output, appConfig.Log.MaxFileSize, appConfig.Log.MaxDays, appConfig.Log.MaxFileNum)
		mw := io.MultiWriter(os.Stdout, logWriter)
		logrus.SetOutput(mw)
	}

	level, err := logrus.ParseLevel(appConfig.Log.Level)

	if err != nil {
		return err
	}

	logrus.SetLevel(level)
	initNodeHealthLogger(appConfig)
	initNodeStatusLogger(appConfig)
	initTaskAssignmentLogger(appConfig)
	initTaskValidationGroupLogger(appConfig)
	initAdminLogger(appConfig)

	return nil
}

func GetNodeHealthLogger() *logrus.Logger {
	return nodeHealthLogger
}

func GetTaskAssignmentLogger() *logrus.Logger {
	return taskAssignmentLogger
}

func GetNodeStatusLogger() *logrus.Logger {
	return nodeStatusLogger
}

func GetTaskValidationGroupLogger() *logrus.Logger {
	return taskValidationGroupLogger
}

func GetAdminLogger() *logrus.Logger {
	return adminLogger
}

func initNodeHealthLogger(appConfig *AppConfig) {
	if !appConfig.Log.Features.NodeHealthEnabled {
		nodeHealthLogger = nil
		return
	}
	nodeHealthLogger = logrus.New()
	nodeHealthLogger.SetFormatter(&logrus.TextFormatter{})
	nodeHealthLogger.SetLevel(logrus.InfoLevel)
	nodeHealthLogger.SetOutput(newLogWriter(getNodeHealthLogPath(appConfig.Log.Output), appConfig.Log.MaxFileSize, appConfig.Log.MaxDays, appConfig.Log.MaxFileNum))
}

func initNodeStatusLogger(appConfig *AppConfig) {
	if !appConfig.Log.Features.NodeStatusEnabled {
		nodeStatusLogger = nil
		return
	}
	nodeStatusLogger = logrus.New()
	nodeStatusLogger.SetFormatter(&logrus.TextFormatter{})
	nodeStatusLogger.SetLevel(logrus.InfoLevel)
	nodeStatusLogger.SetOutput(newLogWriter(getNodeStatusLogPath(appConfig.Log.Output), appConfig.Log.MaxFileSize, appConfig.Log.MaxDays, appConfig.Log.MaxFileNum))
}

func initTaskAssignmentLogger(appConfig *AppConfig) {
	if !appConfig.Log.Features.TaskAssignmentEnabled {
		taskAssignmentLogger = nil
		return
	}
	taskAssignmentLogger = logrus.New()
	taskAssignmentLogger.SetFormatter(&logrus.TextFormatter{})
	taskAssignmentLogger.SetLevel(logrus.InfoLevel)
	taskAssignmentLogger.SetOutput(newLogWriter(getTaskAssignmentLogPath(appConfig.Log.Output), appConfig.Log.MaxFileSize, appConfig.Log.MaxDays, appConfig.Log.MaxFileNum))
}

func initTaskValidationGroupLogger(appConfig *AppConfig) {
	if !appConfig.Log.Features.TaskValidationGroupEnabled {
		taskValidationGroupLogger = nil
		return
	}
	taskValidationGroupLogger = logrus.New()
	taskValidationGroupLogger.SetFormatter(&logrus.TextFormatter{})
	taskValidationGroupLogger.SetLevel(logrus.InfoLevel)
	taskValidationGroupLogger.SetOutput(newLogWriter(getTaskValidationGroupLogPath(appConfig.Log.Output), appConfig.Log.MaxFileSize, appConfig.Log.MaxDays, appConfig.Log.MaxFileNum))
}

func initAdminLogger(appConfig *AppConfig) {
	adminLogger = logrus.New()
	adminLogger.SetFormatter(&logrus.TextFormatter{})
	adminLogger.SetLevel(logrus.InfoLevel)
	adminLogger.SetOutput(newLogWriter(getAdminLogPath(appConfig.Log.Output), appConfig.Log.MaxFileSize, appConfig.Log.MaxDays, appConfig.Log.MaxFileNum))
}

func getNodeHealthLogPath(mainLogOutput string) string {
	if mainLogOutput == "" || mainLogOutput == "stdout" || mainLogOutput == "stderr" {
		return defaultNodeHealthLogPath
	}
	return filepath.Join(filepath.Dir(mainLogOutput), "node_health.log")
}

func getNodeStatusLogPath(mainLogOutput string) string {
	if mainLogOutput == "" || mainLogOutput == "stdout" || mainLogOutput == "stderr" {
		return defaultNodeStatusLogPath
	}
	return filepath.Join(filepath.Dir(mainLogOutput), "node_status.log")
}

func getTaskAssignmentLogPath(mainLogOutput string) string {
	if mainLogOutput == "" || mainLogOutput == "stdout" || mainLogOutput == "stderr" {
		return defaultTaskAssignmentLogPath
	}
	return filepath.Join(filepath.Dir(mainLogOutput), "task_assignment.log")
}

func getTaskValidationGroupLogPath(mainLogOutput string) string {
	if mainLogOutput == "" || mainLogOutput == "stdout" || mainLogOutput == "stderr" {
		return defaultTaskValidationGroupLogPath
	}
	return filepath.Join(filepath.Dir(mainLogOutput), "task_validation_group.log")
}

func getAdminLogPath(mainLogOutput string) string {
	if mainLogOutput == "" || mainLogOutput == "stdout" || mainLogOutput == "stderr" {
		return defaultAdminLogPath
	}
	return filepath.Join(filepath.Dir(mainLogOutput), "admin.log")
}

func newLogWriter(filename string, maxFileSize, maxDays, maxFileNum int) *lumberjack.Logger {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		logrus.WithError(err).Warn("failed to create log directory")
	}
	logWriter := &lumberjack.Logger{
		Filename: filename,
		Compress: true,
	}

	if maxFileSize == 0 {
		logWriter.MaxSize = 500
	} else {
		logWriter.MaxSize = maxFileSize
	}

	if maxDays == 0 {
		logWriter.MaxAge = 30
	} else {
		logWriter.MaxAge = maxDays
	}

	if maxFileNum == 0 {
		logWriter.MaxBackups = 10
	} else {
		logWriter.MaxBackups = maxFileNum
	}

	return logWriter
}
