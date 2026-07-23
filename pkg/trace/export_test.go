package trace

import "sync"

var GlobalTracerProvider = &globalTracerProvider
var GlobalTracer = &globalTracer
var Once = &once

func ResetGlobalsForTest() {
	once = sync.Once{}
	globalTracerProvider = nil
	globalTracer = nil
}
