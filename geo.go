package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Location struct {
	City   string
	Region string // 2-letter state/region code
}

func (l Location) String() string {
	return fmt.Sprintf("%s, %s", l.City, l.Region)
}

func fetchLocation() (Location, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://ip-api.com/json?fields=city,region")
	if err != nil {
		return Location{}, fmt.Errorf("geolocation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Location{}, fmt.Errorf("geolocation request: status %d", resp.StatusCode)
	}

	var result struct {
		City   string `json:"city"`
		Region string `json:"region"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Location{}, fmt.Errorf("parsing geolocation: %w", err)
	}
	return Location{City: result.City, Region: result.Region}, nil
}
