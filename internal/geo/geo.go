// Package geo guesses weather coordinates from the network.
package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// orDefault returns client or a shared default with a bounded timeout; these
// are tiny JSON endpoints and should never hold the wizard open.
func orDefault(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// DefaultEndpoint is a keyless HTTPS IP-geolocation service.
const DefaultEndpoint = "https://ipapi.co/json/"

// DefaultGeocodeEndpoint is the keyless Open-Meteo city search.
const DefaultGeocodeEndpoint = "https://geocoding-api.open-meteo.com/v1/search"

// Place is a guessed location.
type Place struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
}

// Guess asks endpoint where this machine is. GeoClue is a named follow-up.
func Guess(ctx context.Context, client *http.Client, endpoint string) (Place, error) {
	client = orDefault(client)
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

// Search resolves a city name through the Open-Meteo geocoder.
func Search(ctx context.Context, client *http.Client, endpoint, name string) (Place, error) {
	client = orDefault(client)
	u := endpoint + "?name=" + url.QueryEscape(name) + "&count=1&language=en&format=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Place{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Place{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Place{}, fmt.Errorf("geocoding returned %s", resp.Status)
	}
	var body struct {
		Results []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Name      string  `json:"name"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Place{}, err
	}
	if len(body.Results) == 0 {
		return Place{}, fmt.Errorf("no place named %q", name)
	}
	r := body.Results[0]
	if r.Latitude == 0 && r.Longitude == 0 {
		return Place{}, fmt.Errorf("geocoding returned no coordinates for %q", name)
	}
	return Place{Latitude: r.Latitude, Longitude: r.Longitude, City: r.Name}, nil
}
