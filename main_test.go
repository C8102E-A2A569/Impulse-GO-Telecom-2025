package main

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"[10:00:00.000]", "10:00:00.000", false},
		{"[09:30:01.005]", "09:30:01.005", false},
		{"[23:59:59.999]", "23:59:59.999", false},
		{"10:00:00.000", "", true},
		{"[10:00:00]", "", true},
	}

	for _, test := range tests {
		result, err := parseTime(test.input)
		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for input %s, but got none", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input %s: %v", test.input, err)
				continue
			}

			if formatTime(result) != test.expected {
				t.Errorf("For input %s, expected %s, got %s", test.input, test.expected, formatTime(result))
			}
		}
	}
}

func TestParseEventLog(t *testing.T) {
	tests := []struct {
		input         string
		expectedTime  string
		expectedEvent int
		expectedID    int
		expectedExtra string
		hasError      bool
	}{
		{"[09:05:59.867] 1 1", "09:05:59.867", 1, 1, "", false},
		{"[09:15:00.841] 2 1 09:30:00.000", "09:15:00.841", 2, 1, "09:30:00.000", false},
		{"[09:59:03.872] 11 1 Lost in the forest", "09:59:03.872", 11, 1, "Lost in the forest", false},
		{"Invalid event", "", 0, 0, "", true},
	}

	for _, test := range tests {
		result, err := parseEventLog(test.input)
		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for input %s, but got none", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input %s: %v", test.input, err)
				continue
			}

			if formatTime(result.Time) != test.expectedTime {
				t.Errorf("For input %s, expected time %s, got %s", test.input, test.expectedTime, formatTime(result.Time))
			}

			if result.EventID != test.expectedEvent {
				t.Errorf("For input %s, expected event ID %d, got %d", test.input, test.expectedEvent, result.EventID)
			}

			if result.CompetitorID != test.expectedID {
				t.Errorf("For input %s, expected competitor ID %d, got %d", test.input, test.expectedID, result.CompetitorID)
			}

			if result.ExtraParams != test.expectedExtra {
				t.Errorf("For input %s, expected extra params %s, got %s", test.input, test.expectedExtra, result.ExtraParams)
			}
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{1*time.Hour + 30*time.Minute + 45*time.Second + 500*time.Millisecond, "01:30:45.500"},
		{45*time.Second + 5*time.Millisecond, "00:00:45.005"},
		{25*time.Hour + 12*time.Minute + 37*time.Second + 128*time.Millisecond, "25:12:37.128"},
	}

	for _, test := range tests {
		result := formatDuration(test.input)
		if result != test.expected {
			t.Errorf("For input %v, expected %s, got %s", test.input, test.expected, result)
		}
	}
}

func TestCompetitorStats(t *testing.T) {
	config := Configuration{
		Laps:       2,
		LapLen:     3500,
		PenaltyLen: 150,
	}

	competitor := Competitor{
		ID:     1,
		Status: "Finished",
		LapTimes: []time.Duration{
			10 * time.Minute,
			12 * time.Minute,
		},
		TotalPenaltyTime: 2 * time.Minute,
		Hits:             4,
		Shots:            5,
	}

	// Calculate stats
	lapStats, penaltyStats := competitor.calculateStats(config)

	// Check lap stats
	if len(lapStats) != 2 {
		t.Errorf("Expected 2 lap stats, got %d", len(lapStats))
	}

	// Check first lap
	if lapStats[0].Time != "00:10:00.000" {
		t.Errorf("Expected first lap time 00:10:00.000, got %s", lapStats[0].Time)
	}

	expectedSpeed := float64(3500) / (10 * 60)
	if lapStats[0].Speed != expectedSpeed {
		t.Errorf("Expected first lap speed %.3f, got %.3f", expectedSpeed, lapStats[0].Speed)
	}

	// Check penalty stats
	if penaltyStats.Time != "00:02:00.000" {
		t.Errorf("Expected penalty time 00:02:00.000, got %s", penaltyStats.Time)
	}

	expectedPenaltySpeed := float64(150) / (2 * 60)
	if penaltyStats.Speed != expectedPenaltySpeed {
		t.Errorf("Expected penalty speed %.3f, got %.3f", expectedPenaltySpeed, penaltyStats.Speed)
	}
}
