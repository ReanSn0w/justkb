package httpapi

import (
	"encoding/json"
	"errors"
)

// Unlike *bool, this distinguishes an omitted property from explicit null.
type optionalBool struct{ Value *bool }

func (b *optionalBool) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return errors.New("boolean cannot be null")
	}
	var value bool
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	b.Value = &value
	return nil
}
