package stats

import "emby-analytics/internal/media"

var statsMultiMgr *media.MultiServerManager

func SetMultiServerManager(mgr *media.MultiServerManager) {
	statsMultiMgr = mgr
}

func getMultiServerManager() *media.MultiServerManager {
	return statsMultiMgr
}
