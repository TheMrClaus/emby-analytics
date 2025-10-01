package tasks

import "strings"

const (
	serverKeySeparator = "::"
)

func storageUserID(serverID, userID string) string {
	if strings.TrimSpace(userID) == "" {
		return userID
	}
	if serverID == "" || serverID == "default-emby" {
		return userID
	}
	return serverID + serverKeySeparator + userID
}

func storageItemID(serverID, itemID string) string {
	if strings.TrimSpace(itemID) == "" {
		return itemID
	}
	if serverID == "" || serverID == "default-emby" {
		return itemID
	}
	return serverID + serverKeySeparator + itemID
}

func remoteID(serverID, storedID string) string {
	if serverID == "" || serverID == "default-emby" {
		return storedID
	}
	prefix := serverID + serverKeySeparator
	if strings.HasPrefix(storedID, prefix) {
		return strings.TrimPrefix(storedID, prefix)
	}
	return storedID
}

func syncInitializedKey(serverID string) string {
	return "sync_initialized_" + serverID
}
