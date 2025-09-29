package stats

const (
	runtimeOutlierThresholdMinutes = 24 * 60
	runtimeOutlierMaxFixPasses     = 3
)

func isRuntimeOutlier(minutes int) bool {
	return minutes > runtimeOutlierThresholdMinutes
}
