package londo

import "encoding/json"

func UnmarshalMsgBody(b []byte) (Subject, error) {
	var s Subject
	err := json.Unmarshal(b, &s)
	return s, err
}
