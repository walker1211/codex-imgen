package result

import "encoding/json"

func RenderText(res Result) string {
	if res.OK {
		return res.Path + "\n"
	}
	return res.Error + "\n"
}

func RenderJSON(res Result) string {
	data, err := json.Marshal(res)
	if err != nil {
		panic(err)
	}
	return string(data) + "\n"
}
