package log

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"time"
)

var logfile os.File

//There is the possibility to implement a custom HooK

func LogSetup(logLevel logrus.Level) {
	file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Error opening file: ", err)
	}
	logfile = *file

	multiWriter := io.MultiWriter(os.Stdout, file)
	logrus.SetOutput(multiWriter)
	logrus.SetReportCaller(true)

	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat:        time.DateTime,
		FullTimestamp:          true,
		DisableLevelTruncation: true,
		DisableSorting:         false,
	})

	//logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logLevel)
}

func LogClose() {
	// Assuming 'file' is the file handler being used in LogSetup.
	err := logfile.Close()
	if err != nil {
		fmt.Println("Error closing file: ", err)
	}
}
