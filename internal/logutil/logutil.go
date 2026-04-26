package logutil

import (
	"fmt"
	"os"
	"time"
)

func Stamp(now time.Time, message string) string {
	return fmt.Sprintf("[%s] %s", now.Format(time.RFC3339), message)
}

func Println(message string) {
	fmt.Fprintln(os.Stdout, Stamp(time.Now(), message))
}

func Printf(format string, args ...any) {
	fmt.Fprintln(os.Stdout, Stamp(time.Now(), fmt.Sprintf(format, args...)))
}

func Warnf(format string, args ...any) {
	stderrf(format, args...)
}

func Errorf(format string, args ...any) {
	stderrf(format, args...)
}

func stderrf(format string, args ...any) {
	fmt.Fprintln(os.Stderr, Stamp(time.Now(), fmt.Sprintf(format, args...)))
}
