package beeper

import "encoding/json"

func jsonUnmarshalStrings(raw string, target *[]string) error {
	return json.Unmarshal([]byte(raw), target)
}
