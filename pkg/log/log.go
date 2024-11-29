package log

import (
	"fmt"
	
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
)

func NewStdoutLogger() logr.Logger {
	return funcr.New(func(prefix, args string) {
		if prefix != "" {
			fmt.Printf("%s: %s\n", prefix, args)
		} else {
			fmt.Printf("%s\n", args)
		}
	}, funcr.Options{})
}
