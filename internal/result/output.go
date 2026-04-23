package result

import (
	"encoding/json"
	"strings"
)

func RenderText(res Result) string {
	if !res.OK {
		return res.Error + "\n"
	}
	if len(res.Images) == 0 {
		if res.JobID != "" {
			return res.JobID + " " + res.Status + "\n"
		}
		return ""
	}
	var lines []string
	for _, image := range res.Images {
		if image.Path != "" {
			lines = append(lines, image.Path)
		}
	}
	if len(lines) > 0 {
		return strings.Join(lines, "\n") + "\n"
	}
	if res.JobID != "" {
		return res.JobID + " " + res.Status + "\n"
	}
	return ""
}

func RenderJSON(res Result) string {
	data, err := json.Marshal(res)
	if err != nil {
		panic(err)
	}
	return string(data) + "\n"
}
