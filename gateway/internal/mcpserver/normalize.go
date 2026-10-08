package mcpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type normalizedPlace struct {
	Rank               *int              `json:"rank,omitempty"`
	ID                 string            `json:"id,omitempty"`
	Name               string            `json:"name,omitempty"`
	Category           json.RawMessage   `json:"category,omitempty"`
	RoadAddress        string            `json:"road_address,omitempty"`
	Coordinates        *placeCoordinates `json:"coordinates,omitempty"`
	Tel                string            `json:"tel,omitempty"`
	BusinessStatus     string            `json:"business_status,omitempty"`
	BusinessHours      string            `json:"business_hours,omitempty"`
	BreakTime          string            `json:"break_time,omitempty"`
	LastOrder          string            `json:"last_order,omitempty"`
	ThumbnailURL       string            `json:"thumbnail_url,omitempty"`
	Homepage           string            `json:"homepage,omitempty"`
	MenuInfo           string            `json:"menu_info,omitempty"`
	ReservationOptions []string          `json:"reservation_options,omitempty"`
}

type placeCoordinates struct {
	Longitude *float64 `json:"longitude,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
}

type upstreamBusinessStatus struct {
	Status struct {
		Text        string `json:"text"`
		Description string `json:"description"`
	} `json:"status"`
	BusinessHours string `json:"businessHours"`
	BreakTime     string `json:"breakTime"`
	LastOrder     string `json:"lastOrder"`
}

type upstreamReservationLabels struct {
	Standard bool `json:"standard"`
	PreOrder bool `json:"preOrder"`
	Table    bool `json:"table"`
	Takeout  bool `json:"takeout"`
}

// normalizeNaverResults maps Naver's changing place records to the compact MCP schema.
func normalizeNaverResults(rawJSON []byte) ([]byte, error) {
	trimmedJSON := bytes.TrimSpace(rawJSON)
	if len(trimmedJSON) == 0 || trimmedJSON[0] != '[' {
		return nil, fmt.Errorf("Naver place results must be a JSON array")
	}
	var upstream []map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON, &upstream); err != nil {
		return nil, fmt.Errorf("decode Naver place result list: %w", err)
	}

	results := make([]normalizedPlace, 0, len(upstream))
	for index, fields := range upstream {
		if fields == nil {
			return nil, fmt.Errorf("Naver place result %d is not an object", index)
		}
		results = append(results, normalizeNaverPlace(fields))
	}

	normalized, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("encode normalized Naver place results: %w", err)
	}
	return normalized, nil
}

func normalizeNaverPlace(fields map[string]json.RawMessage) normalizedPlace {
	place := normalizedPlace{
		ID:           rawText(fields, "id"),
		Name:         rawText(fields, "name"),
		RoadAddress:  rawText(fields, "roadAddress"),
		Tel:          firstRawText(fields, "tel", "virtualTel"),
		ThumbnailURL: rawText(fields, "thumUrl"),
		Homepage:     rawText(fields, "homePage"),
		MenuInfo:     rawText(fields, "menuInfo"),
	}

	if rank, ok := rawInt(fields["rank"]); ok {
		place.Rank = &rank
	}
	if category, ok := nonEmptyJSON(fields["category"]); ok {
		place.Category = category
	}
	longitude, hasLongitude := rawFloat(fields["x"])
	latitude, hasLatitude := rawFloat(fields["y"])
	if hasLongitude || hasLatitude {
		place.Coordinates = &placeCoordinates{}
		if hasLongitude {
			place.Coordinates.Longitude = &longitude
		}
		if hasLatitude {
			place.Coordinates.Latitude = &latitude
		}
	}

	var business upstreamBusinessStatus
	if json.Unmarshal(fields["businessStatus"], &business) == nil {
		place.BusinessStatus = firstNonEmpty(business.Status.Text, business.Status.Description)
		place.BusinessHours = formatTimeRange(business.BusinessHours)
		place.BreakTime = formatTimeRange(business.BreakTime)
		place.LastOrder = formatTime(business.LastOrder)
	}

	var reservation upstreamReservationLabels
	if json.Unmarshal(fields["reservationLabel"], &reservation) == nil {
		if reservation.Standard {
			place.ReservationOptions = append(place.ReservationOptions, "reservation")
		}
		if reservation.PreOrder {
			place.ReservationOptions = append(place.ReservationOptions, "pre-order")
		}
		if reservation.Table {
			place.ReservationOptions = append(place.ReservationOptions, "table")
		}
		if reservation.Takeout {
			place.ReservationOptions = append(place.ReservationOptions, "takeout")
		}
	}

	return place
}

func rawText(fields map[string]json.RawMessage, key string) string {
	var value string
	if len(fields[key]) == 0 || json.Unmarshal(fields[key], &value) != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func firstRawText(fields map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		if value := rawText(fields, key); value != "" {
			return value
		}
	}
	return ""
}

func rawInt(value json.RawMessage) (int, bool) {
	text := strings.TrimSpace(string(value))
	if len(text) == 0 {
		return 0, false
	}
	if text[0] == '"' {
		if err := json.Unmarshal(value, &text); err != nil {
			return 0, false
		}
	}
	parsed, err := strconv.Atoi(text)
	return parsed, err == nil
}

func rawFloat(value json.RawMessage) (float64, bool) {
	text := strings.TrimSpace(string(value))
	if len(text) == 0 {
		return 0, false
	}
	if text[0] == '"' {
		if err := json.Unmarshal(value, &text); err != nil {
			return 0, false
		}
	}
	parsed, err := strconv.ParseFloat(text, 64)
	return parsed, err == nil
}

func nonEmptyJSON(value json.RawMessage) (json.RawMessage, bool) {
	if len(value) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, false
	}
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return nil, false
	}
	switch decoded := decoded.(type) {
	case string:
		if strings.TrimSpace(decoded) == "" {
			return nil, false
		}
	case []any:
		if len(decoded) == 0 {
			return nil, false
		}
	case map[string]any:
		if len(decoded) == 0 {
			return nil, false
		}
	}
	return append(json.RawMessage(nil), value...), true
}

func formatTimeRange(value string) string {
	parts := strings.Split(value, "~")
	if len(parts) != 2 {
		return ""
	}
	start, end := formatTime(parts[0]), formatTime(parts[1])
	if start == "" || end == "" {
		return ""
	}
	return start + "–" + end
}

func formatTime(value string) string {
	digits := make([]rune, 0, len(value))
	for _, char := range value {
		if char >= '0' && char <= '9' {
			digits = append(digits, char)
		}
	}
	if len(digits) == 12 {
		digits = digits[8:]
	}
	if len(digits) != 4 {
		return ""
	}
	hours, _ := strconv.Atoi(string(digits[:2]))
	minutes, _ := strconv.Atoi(string(digits[2:]))
	if hours > 24 || minutes > 59 || (hours == 24 && minutes != 0) {
		return ""
	}
	return string(digits[:2]) + ":" + string(digits[2:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
