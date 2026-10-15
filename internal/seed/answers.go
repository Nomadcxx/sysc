package seed

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// AnswersFromConfig recovers the answers an update needs from a config the
// installer already wrote. Only the weather block is read: it is the one the
// pipeline requires and cannot invent, and copying it means an update cannot
// start from weaker inputs than a fresh install. Everything else keeps the same
// defaults answersFor uses.
func AnswersFromConfig(path string) (Answers, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Answers{}, fmt.Errorf("read %s: %w", path, err)
	}
	var wire struct {
		Weather struct {
			Latitude  *float64 `json:"latitude"`
			Longitude *float64 `json:"longitude"`
			Location  string   `json:"location"`
		} `json:"weather"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return Answers{}, fmt.Errorf("parse %s: %w", path, err)
	}
	w := wire.Weather
	if w.Latitude == nil || w.Longitude == nil || w.Location == "" {
		return Answers{}, errors.New("the shell config has no complete weather block; set the weather widget once in the shell's settings, or run sysc and answer the questions")
	}
	return Answers{Latitude: *w.Latitude, Longitude: *w.Longitude, Location: w.Location}, nil
}
