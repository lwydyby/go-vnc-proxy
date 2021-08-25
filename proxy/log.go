package proxy

import (
	"bytes"
	"context"
	log "github.com/lwydyby/logrus"
	"runtime"
	"strconv"
)

func AddTraceIdHook(traceId string) log.Hook {
	traceHook := newTraceIdHook(traceId)
	if log.StandardLogger().Hooks == nil {
		hooks := new(log.LevelHooks)
		log.StandardLogger().ReplaceHooks(*hooks)
	}
	log.AddHook(traceHook)
	return traceHook
}

func RemoveTraceHook(hook log.Hook) {
	allHooks := log.StandardLogger().Hooks
	func() {
		defer log.Unlock()
		log.Lock()
		for key, hooks := range allHooks {
			replaceHooks := hooks
			for index, h := range hooks {
				if h == hook {
					replaceHooks = append(hooks[:index], hooks[index:]...)
					break
				}
			}
			allHooks[key] = replaceHooks
		}
	}()
	log.StandardLogger().ReplaceHooks(allHooks)
}

type TraceIdHook struct {
	TraceId string
	GID     uint64
}

func newTraceIdHook(traceId string) log.Hook {
	return &TraceIdHook{
		TraceId: traceId,
		GID:     getGID(),
	}
}

func (t TraceIdHook) Levels() []log.Level {
	return log.AllLevels
}

func (t TraceIdHook) Fire(entry *log.Entry) error {
	if getGID() == t.GID {
		entry.Context = context.WithValue(context.Background(), "trace_id", t.TraceId)
	}
	return nil
}

// 获取当前协程id
func getGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
