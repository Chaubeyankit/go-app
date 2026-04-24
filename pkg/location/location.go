package location

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Location represents geographical location information
type Location struct {
	City       string `json:"city"`
	Country    string `json:"country"`
	CountryCode string `json:"countryCode"`
	Region     string `json:"region"`
	RegionCode string `json:"regionCode"`
	Timezone   string `json:"timezone"`
	Latitude   string `json:"latitude"`
	Longitude  string `json:"longitude"`
	IP         string `json:"ip"`
}

// Detector provides IP location detection functionality
type Detector struct {
	httpClient *http.Client
	apiKey    string
	apiURL    string
}

// NewDetector creates a new location detector
func NewDetector(apiKey, apiURL string) *Detector {
	return &Detector{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		apiKey: apiKey,
		apiURL: apiURL,
	}
}

// DetectLocation detects the location of an IP address
func (d *Detector) DetectLocation(ctx context.Context, ip string) (*Location, error) {
	// Skip location detection for localhost/private IPs
	if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") {
		return &Location{
			IP:         ip,
			City:       "Local",
			Country:    "Local",
			CountryCode: "LOCAL",
			Region:     "Local",
		}, nil
	}

	// If no API key is provided, return basic info
	if d.apiKey == "" {
		return &Location{
			IP:         ip,
			Country:    "Unknown",
			CountryCode: "UNK",
			City:       "Unknown",
			Region:     "Unknown",
		}, nil
	}

	// Use IPInfo.io API
	url := fmt.Sprintf("https://ipinfo.io/%s/json", ip)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add API key to request
	req.Header.Add("Authorization", "Bearer "+d.apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &Location{
			IP:         ip,
			Country:    "Unknown",
			CountryCode: "UNK",
			City:       "Unknown",
			Region:     "Unknown",
		}, nil
	}

	// Parse the response
	var result struct {
		IP          string  `json:"ip"`
		City        string  `json:"city"`
		Region      string  `json:"region"`
		Country     string  `json:"country"`
		CountryCode string  `json:"countryCode"`
		Loc         string  `json:"loc"`
		Timezone    string  `json:"timezone"`
		Org         string  `json:"org"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Parse location coordinates if available
	var latitude, longitude string
	if result.Loc != "" {
		parts := strings.Split(result.Loc, ",")
		if len(parts) == 2 {
			latitude = parts[0]
			longitude = parts[1]
		}
	}

	return &Location{
		IP:         result.IP,
		City:       result.City,
		Country:    result.Country,
		CountryCode: result.CountryCode,
		Region:     result.Region,
		RegionCode: result.Region,
		Timezone:   result.Timezone,
		Latitude:   latitude,
		Longitude:  longitude,
	}, nil
}

// DetectBrowser extracts browser information from User-Agent
func DetectBrowser(userAgent string) string {
	userAgent = strings.ToLower(userAgent)

	if strings.Contains(userAgent, "chrome") && !strings.Contains(userAgent, "edge") {
		if strings.Contains(userAgent, "mobile") {
			return "Chrome Mobile"
		}
		return "Chrome"
	}

	if strings.Contains(userAgent, "firefox") {
		if strings.Contains(userAgent, "mobile") {
			return "Firefox Mobile"
		}
		return "Firefox"
	}

	if strings.Contains(userAgent, "safari") && !strings.Contains(userAgent, "chrome") {
		if strings.Contains(userAgent, "mobile") {
			return "Safari Mobile"
		}
		return "Safari"
	}

	if strings.Contains(userAgent, "edge") {
		return "Edge"
	}

	if strings.Contains(userAgent, "opera") {
		return "Opera"
	}

	return "Unknown Browser"
}

// DetectOS extracts operating system information from User-Agent
func DetectOS(userAgent string) string {
	userAgent = strings.ToLower(userAgent)

	if strings.Contains(userAgent, "windows") {
		if strings.Contains(userAgent, "phone") {
			return "Windows Phone"
		}
		return "Windows"
	}

	if strings.Contains(userAgent, "mac os") {
		return "macOS"
	}

	if strings.Contains(userAgent, "linux") {
		if strings.Contains(userAgent, "android") {
			return "Android"
		}
		return "Linux"
	}

	if strings.Contains(userAgent, "android") {
		return "Android"
	}

	if strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad") {
		return "iOS"
	}

	return "Unknown OS"
}