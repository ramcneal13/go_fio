package support

import "testing"

func TestStatsAdd(t *testing.T) {
	ws := WorkerStat{}
	ws.Histogram = &DistroGraph{}
	ws.BytesRead = 100
	ss := SlaveState{}
	ss.totalStats.Histogram = &DistroGraph{}
	ss.totalStats.BytesRead = 13
	DebugEnable()
	ss.addStats(&ws)
	DebugDisable()
	if ss.totalStats.BytesRead != 113 {
		t.Errorf("BytesRead(%d) != 113\n", ss.totalStats.BytesRead)
	}
}
