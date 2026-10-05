// Package geo guesses weather coordinates from the network.
package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DefaultEndpoint is a keyless HTTPS IP-geolocation service.
const DefaultEndpoint = "https://ipapi.co/json/"

// Place is a guessed location.
type Place struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
}

// Guess asks endpoint where this machine is. GeoClue is a named follow-up.
func Guess(ctx context.Context, client *http.Client, endpoint string) (Place, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Place{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Place{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Place{}, fmt.Errorf("geolocation returned %s", resp.Status)
	}
	var p Place
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return Place{}, err
	}
	if p.Latitude == 0 && p.Longitude == 0 {
		return Place{}, fmt.Errorf("geolocation returned no coordinates")
	}
	return p, nil
}
