package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

func Init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	log = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()
}

func Info(message string, fields ...map[string]interface{}) {
	event := log.Info()
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(message)
}

func Error(err error, message string, fields ...map[string]interface{}) {
	event := log.Error().Err(err)
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(message)
}

func Debug(message string, fields ...map[string]interface{}) {
	event := log.Debug()
	if len(fields) > 0 {
		for key, value := range fields[0] {
			event = event.Interface(key, value)
		}
	}
	event.Msg(message)
}
