package mcpserver

import (
	"encoding/json"
	"testing"
)

func TestNormalizeNaverResults(t *testing.T) {
	raw := []byte(`[
		{
			"rank":"1",
			"id":"place-1",
			"name":"Example Cafe",
			"category":["카페"],
			"roadAddress":"서울시 도로명 1",
			"address":"서울시 지번 1",
			"x":"127.1",
			"y":"37.5",
			"tel":"",
			"virtualTel":"02-1234-5678",
			"thumUrl":"https://example.test/thumb.jpg",
			"homePage":"https://example.test",
			"menuInfo":"Coffee 5,000",
			"businessStatus":{
				"status":{"text":"영업 중"},
				"businessHours":"202610081100~202610082200",
				"breakTime":"202610081500~202610081700",
				"lastOrder":"202610082100"
			},
			"reservationLabel":{"standard":true,"preOrder":false,"table":true,"takeout":false},
			"reviewCount":12,
			"placeReviewCount":34,
			"distance":"100",
			"indoor":null,
			"subway":null
		},
		{
			"rank":2,
			"id":"place-2",
			"name":"No Optional Values",
			"category":"",
			"tel":"",
			"virtualTel":"",
			"businessStatus":{"businessHours":"","breakTime":"","lastOrder":""},
			"reservationLabel":{"standard":false,"preOrder":false,"table":false,"takeout":false}
		}
	]`)

	normalized, err := normalizeNaverResults(raw)
	if err != nil {
		t.Fatalf("normalizeNaverResults() error = %v", err)
	}

	var results []map[string]any
	if err := json.Unmarshal(normalized, &results); err != nil {
		t.Fatalf("normalized JSON is invalid: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	first := results[0]
	if first["rank"] != float64(1) || first["tel"] != "02-1234-5678" {
		t.Fatalf("rank or telephone was not normalized: %#v", first)
	}
	if first["road_address"] != "서울시 도로명 1" {
		t.Fatalf("road address was not preferred: %#v", first["road_address"])
	}
	if first["business_hours"] != "11:00–22:00" || first["break_time"] != "15:00–17:00" || first["last_order"] != "21:00" {
		t.Fatalf("business times were not formatted: %#v", first)
	}
	if first["business_status"] != "영업 중" || first["thumbnail_url"] != "https://example.test/thumb.jpg" || first["homepage"] != "https://example.test" || first["menu_info"] != "Coffee 5,000" {
		t.Fatalf("place details were not mapped: %#v", first)
	}
	if got := first["reservation_options"]; !equalJSONValue(got, []any{"reservation", "table"}) {
		t.Fatalf("reservation options = %#v, want reservation and table", got)
	}
	coordinates := first["coordinates"].(map[string]any)
	if coordinates["longitude"] != 127.1 || coordinates["latitude"] != 37.5 {
		t.Fatalf("coordinates were not normalized: %#v", coordinates)
	}
	for _, excluded := range []string{"address", "reviewCount", "placeReviewCount", "distance", "indoor", "subway", "virtualTel", "businessStatus", "reservationLabel"} {
		if _, exists := first[excluded]; exists {
			t.Errorf("excluded upstream field %q was returned", excluded)
		}
	}

	second := results[1]
	for _, optional := range []string{"category", "tel", "business_status", "business_hours", "break_time", "last_order", "reservation_options"} {
		if _, exists := second[optional]; exists {
			t.Errorf("empty optional field %q was returned", optional)
		}
	}
}

func TestNormalizeNaverResultsRejectsNonArray(t *testing.T) {
	if _, err := normalizeNaverResults([]byte(`{"result":[]}`)); err == nil {
		t.Fatal("normalizeNaverResults() error = nil, want a non-array error")
	}
}

func equalJSONValue(actual any, expected []any) bool {
	actualValues, ok := actual.([]any)
	if !ok || len(actualValues) != len(expected) {
		return false
	}
	for index := range expected {
		if actualValues[index] != expected[index] {
			return false
		}
	}
	return true
}
