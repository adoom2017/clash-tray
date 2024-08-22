package log

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logConfig = LogConfig{
	Level:      INFO,
	Filename:   "",
	MaxSize:    10,
	MaxAge:     3,
	MaxBackups: 0,
	Compress:   false,
}

type LogConfig struct {
	Level      LogLevel
	Filename   string
	MaxSize    int
	MaxAge     int
	MaxBackups int
	Compress   bool
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&Formatter{
		HideKeys:    true,
		CallerFirst: true,
	})
}

func GetLogConfig() LogConfig {
	return logConfig
}

func SetLogConfig(config LogConfig) {
	if config.Filename != "" {
		dir := filepath.Dir(config.Filename)

		// Create the log directory if it doesn't exist
		_, err := os.Stat(dir)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				log.Errorf("Failed to create dir: %s, err: %v", dir, err)
				return
			}
		}

		// Set the log output info
		log.SetOutput(&lumberjack.Logger{
			Filename:   config.Filename,
			MaxSize:    config.MaxSize,
			MaxAge:     config.MaxAge,
			MaxBackups: config.MaxBackups,
			Compress:   config.Compress,
		})
	}

	logConfig = config
}

func Infoln(format string, v ...any) {
	print(INFO, fmt.Sprintf(format, v...))
}

func Warnln(format string, v ...any) {
	print(WARNING, fmt.Sprintf(format, v...))
}

func Errorln(format string, v ...any) {
	print(ERROR, fmt.Sprintf(format, v...))
}

func Debugln(format string, v ...any) {
	print(DEBUG, fmt.Sprintf(format, v...))
}

func Fatalln(format string, v ...any) {
	log.Fatalf(format, v...)
}

func print(level LogLevel, data string) {
	if level < logConfig.Level {
		return
	}

	switch level {
	case INFO:
		log.Infoln(data)
	case WARNING:
		log.Warnln(data)
	case ERROR:
		log.Errorln(data)
	case DEBUG:
		log.Debugln(data)
	}
}
