package logger

import "sync"

var MapLevel = mapLevel
var ResolveLogPath = resolveLogPath

func ResetGlobalsForTest() {
	once = sync.Once{}
	globalLogger = nil
	globalWriter = nil
}
