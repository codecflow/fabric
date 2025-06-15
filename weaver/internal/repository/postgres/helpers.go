package postgres

import (
	"encoding/json"
)

// toJSON converts a value to JSON bytes
func toJSON(v interface{}) []byte {
	if v == nil {
		return []byte("{}")
	}

	data, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}

	return data
}

// fromJSON converts JSON bytes to a value
func fromJSON(data []byte, v interface{}) error {
	if len(data) == 0 || string(data) == "{}" {
		return nil
	}

	return json.Unmarshal(data, v)
}
