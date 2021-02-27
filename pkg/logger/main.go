package logger

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"k8s.io/klog/v2"

	shared "github.com/iLert/ilert-kube-agent"
	"github.com/iLert/ilert-kube-agent/pkg/config"
)

// Init initializes logs
func Init(setting config.ConfigSettingsLog) {
	if !setting.JSON {
		log.Logger = log.With().Caller().Logger().Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			NoColor:    true,
			PartsOrder: []string{"time", "message", "caller", "level"},
			FormatLevel: func(i interface{}) string {
				return strings.ToLower(fmt.Sprintf("level=%s", i))
			},
		})
	}
	switch setting.Level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Logger = log.With().Str("version", shared.Version).Str("app", shared.App).Logger()
	klog.SetOutput(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    true,
		PartsOrder: []string{"time", "message", "caller", "level"},
		FormatLevel: func(i interface{}) string {
			return strings.ToLower(fmt.Sprintf("level=%s", i))
		},
	})
	log.Debug().Msg("Logger initialized")
}
