package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/consts"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ubuntu/decorate"
)

func initViperConfig(name string, cmd *cobra.Command, vip *viper.Viper) (err error) {
	defer decorate.OnError(&err, "can't load configuration")

	// Use command-line flag for verbosity until configuration is parsed
	v, err := cmd.Flags().GetCount("verbosity")
	if err != nil {
		return fmt.Errorf("internal error: no persistent verbosity flags installed on cmd: %w", err)
	}
	setVerboseMode(v)

	// Find a valid configuration file
	if v, err := cmd.Flags().GetString("config"); err == nil && v != "" {
		vip.SetConfigFile(v)
	} else {
		vip.SetConfigName(name)
		vip.AddConfigPath("./")
		vip.AddConfigPath("$HOME/")
		vip.AddConfigPath("/etc/")
		if binPath, err := os.Executable(); err != nil {
			log.Warningf(context.Background(), "Failed to get the current executable path, not adding it as a config dir: %v", err)
		} else {
			vip.AddConfigPath(filepath.Dir(binPath))
		}
	}

	// Load the config
	if err := vip.ReadInConfig(); err != nil {
		var e viper.ConfigFileNotFoundError
		if errors.As(err, &e) {
			log.Infof(context.Background(), "No configuration file: %v", e)
		} else {
			return fmt.Errorf("invalid configuration file: %v", err)
		}
	} else {
		log.Infof(context.Background(), "Using configuration file: %v", vip.ConfigFileUsed())
	}

	// Parse environment variables
	vip.SetEnvPrefix("UP4W")
	vip.AutomaticEnv()

	return nil
}

// installVerbosityFlag adds the -v and -vv options and returns the reference to it.
func installVerbosityFlag(cmd *cobra.Command, viper *viper.Viper) *int {
	r := cmd.PersistentFlags().CountP("verbosity", "v", i18n.G("issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output"))
	if err := viper.BindPFlag("verbosity", cmd.PersistentFlags().Lookup("verbosity")); err != nil {
		log.Warning(context.Background(), err)
	}
	return r
}

// installConfigFlag adds the --config flag to allow for custom config paths.
func installConfigFlag(cmd *cobra.Command, viper *viper.Viper) *string {
	return cmd.PersistentFlags().StringP("config", "c", "", i18n.G("configuration file path"))
}

// SetVerboseMode change ErrorFormat and logs between very, middly and non verbose.
func setVerboseMode(level int) {
	var reportCaller bool
	switch level {
	case 0:
		logrus.SetLevel(consts.DefaultLogLevel)
	case 1:
		logrus.SetLevel(logrus.InfoLevel)
	case 3:
		reportCaller = true
		fallthrough
	default:
		logrus.SetLevel(logrus.DebugLevel)
	}
	log.SetReportCaller(reportCaller)
}
