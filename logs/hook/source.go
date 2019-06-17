package hook

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// Trace:
// log caller -> logrus.[Entry,Logger][Info,others](maybe twice or more)
// -> logrus.Entry.log -> logrus.Entry.fireHooks -> logrus.LevelHooks.Fire -> logrus.Fire
// So, we just skip all the functions that in logrus and catch the first one after logrus

const (
	// Tricky value according to trace
	callerTrickySkipping = 5
	callerTraceDepth     = 3
	callerFieldName      = "caller"
	pathSeparator        = string(os.PathSeparator)
)

type Source struct {
	level            logrus.Level
	srcPath          string
	useShortenCaller bool
}

func NewSource(level logrus.Level) *Source {
	return &Source{
		level:            level,
		srcPath:          filepath.Join(build.Default.GOPATH, "src"),
		useShortenCaller: true,
	}
}

func (s *Source) Fire(entry *logrus.Entry) error {
	trace := make([]uintptr, callerTraceDepth)
	actualDepth := runtime.Callers(callerTrickySkipping, trace)
	if actualDepth == 0 {
		return nil
	}
	frames := runtime.CallersFrames(trace[:actualDepth])
	for {
		current, next := frames.Next()
		// Skipping all the stack contains logrus
		if strings.Contains(current.File, "github.com/sirupsen/logrus") {
			// Oops, hit the bottom of stack we have to break the loop
			if !next {
				break
			}

			// Keep going
			continue
		}
		// Catch first frame after logrus, construct field
		entry.Data[callerFieldName] = s.makeSourceField(current)
		break
	}

	return nil
}

func (s *Source) Levels() []logrus.Level {
	levels := make([]logrus.Level, 0)
	for _, level := range logrus.AllLevels {
		if level <= s.level {
			levels = append(levels, level)
		}
	}
	return levels
}

// Format ->  file:line(function)
func (s *Source) makeSourceField(frame runtime.Frame) string {
	funcSlice := strings.Split(frame.Function, ".")
	funcName := funcSlice[len(funcSlice)-1:][0]
	fileName := "<file>"
	if s.useShortenCaller {
		paths := strings.Split(frame.File, pathSeparator)
		if len(paths) > 0 {
			fileName = paths[len(paths)-1]
		}
	} else {
		fileName = strings.TrimPrefix(frame.File, s.srcPath+pathSeparator)
	}
	return fmt.Sprintf("%s:%d(%s)", fileName, frame.Line, funcName)
}
