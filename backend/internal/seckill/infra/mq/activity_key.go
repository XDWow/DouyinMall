package mq

import "strconv"

func activityKey(activityID int64) string {
	return strconv.FormatInt(activityID, 10)
}
