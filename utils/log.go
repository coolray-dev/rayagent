package utils

import (
	"github.com/sirupsen/logrus"
)

// Log is a public logger of whole project
var Log *logrus.Logger

func init() {
	Log = logrus.New()
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:          true,
		DisableLevelTruncation: true,
		PadLevelText:           true,
	})
	// Consider not to use Config here in case Config has not been initialized
	// Also to avoid import circle
}
