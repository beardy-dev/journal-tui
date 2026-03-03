package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Location struct {
	City   string
	Region string // 2-letter state/region code
}

func (l Location) String() string {
	if l.City == "" && l.Region == "" {
		return ""
	}
	if l.City == "" {
		return l.Region
	}
	if l.Region == "" {
		return l.City
	}
	return fmt.Sprintf("%s, %s", l.City, l.Region)
}

func fetchLocation() (Location, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	providers := []string{
		"https://ipapi.co/json/",
		"https://ipinfo.io/json",
	}
	var errs []string

	for _, url := range providers {
		loc, err := fetchLocationFrom(client, url)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", url, err))
			continue
		}
		if loc.String() != "" {
			return loc, nil
		}
		errs = append(errs, fmt.Sprintf("%s: empty location", url))
	}

	return Location{}, fmt.Errorf("geolocation lookup failed: %s", strings.Join(errs, "; "))
}

func fetchLocationFrom(client *http.Client, url string) (Location, error) {
	resp, err := client.Get(url)
	if err != nil {
		return Location{}, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Location{}, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		City       string `json:"city"`
		Region     string `json:"region"`
		RegionCode string `json:"region_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Location{}, fmt.Errorf("parse: %w", err)
	}
	region := strings.TrimSpace(result.RegionCode)
	if region == "" {
		region = strings.TrimSpace(result.Region)
	}
	return Location{
		City:   strings.TrimSpace(result.City),
		Region: region,
	}, nil
}
