package store

import "time"

func unixTime(v int64) time.Time {
	return time.Unix(v, 0)
}
