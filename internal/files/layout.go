package files

import (
	"path/filepath"
	"strconv"
)

type Layout struct {
	DataDir string
}

func (l Layout) JobDir(jobID string) string {
	return filepath.Join(l.DataDir, "jobs", jobID)
}

func (l Layout) ImagePath(jobID string, index int) string {
	return filepath.Join(l.JobDir(jobID), "images", strconv.Itoa(index)+".png")
}
