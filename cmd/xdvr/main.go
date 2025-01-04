package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
)

// Support OS-specific path separators.
const slash = string(os.PathSeparator)

var (
	cmd = &cobra.Command{
		Use:     "xdvr",
		Short:   "Download clips and screenshots from your Xbox account.",
		Version: "0.1.1",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			var logWriter io.Writer

			if prettyLog {
				logWriter = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
			} else {
				logWriter = os.Stdout
			}

			logFile, err := os.OpenFile("xdvr.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
			if err != nil {
				panic(err)
			}

			if debug {
				log = zerolog.New(zerolog.MultiLevelWriter(logWriter, logFile)).With().Timestamp().Logger().Level(zerolog.DebugLevel)
			} else {
				log = zerolog.New(zerolog.MultiLevelWriter(logWriter, logFile)).With().Timestamp().Logger().Level(zerolog.InfoLevel)
			}

			configDir, err := os.UserHomeDir()
			if err != nil {
				log.Fatal().Err(err).Msg("unable to retrieve user's home directory")
			}

			cfg, err = NewConfig(fmt.Sprintf("%s%s.config%sxbox-dvr", strings.TrimSuffix(configDir, slash), slash, slash))
			if err != nil {
				log.Fatal().Err(err).Msg("unable to initialize config")
			}

			if debug {
				log.Info().Msg("Debug mode enabled")
				log.Info().Msg("Auto delete: " + cast.ToString(cfg.AutoDelete))
				log.Info().Msg("Save path: " + cfg.SavePath)
				log = log.Level(zerolog.DebugLevel)
			}
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if logFile != nil {
				if err := logFile.Close(); err != nil {
					fmt.Println("failed to gracefully close log file")
					panic(err)
				}
			}
		},
	}

	cfg              *Config
	log              zerolog.Logger
	logFile          *os.File
	debug, prettyLog bool
	timeout          time.Duration
)

func main() {
	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Print debug logs")
	if val, ok := os.LookupEnv("debug"); ok {
		debug = cast.ToBool(val)
	}

	cmd.PersistentFlags().BoolVar(&prettyLog, "prettyLog", true, "Pretty print logs to console")
	if val, ok := os.LookupEnv("prettyLog"); ok {
		prettyLog = cast.ToBool(val)
	}

	cmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 60*time.Second, "Timeout duration for queries")
	if val, ok := os.LookupEnv("timeout"); ok {
		timeout = cast.ToDuration(val)
	}

	_ = cmd.Execute()
}
