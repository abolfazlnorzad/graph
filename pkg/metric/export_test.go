package metric

import "sync"

var GlobalMeterProvider = &globalMeterProvider
var GlobalMeter = &globalMeter
var Once = &once

func ResetGlobalsForTest() {
	once = sync.Once{}
	globalMeterProvider = nil
	globalMeter = nil
}
