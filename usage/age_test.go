package usage

import (
	"strings"
	"testing"
	"time"
)

func birthdate(t *testing.T, value string) int64 {
	t.Helper()
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatal(err)
	}
	return date.Unix()
}

// risingObservations gives a player six games that the model flags as rising.
func risingObservations(playerID string) []Observation {
	var observations []Observation
	for week, snaps := range []int{12, 12, 12, 36, 42, 48} {
		observations = append(observations, Observation{
			PlayerID: playerID, GameID: playerID + "-" + string(rune('a'+week)), Season: 2026, Week: week + 1,
			GameType: "REG", DefenseSnaps: snaps, TeamDefenseSnaps: 60, DefenseSnapPct: float64(snaps) / 60,
		})
	}
	return observations
}

func TestAnalyzeAgeFilter(t *testing.T) {
	// Ages are computed on 2026-11-21.
	players := []Player{
		{ID: "age-27", Name: "Twenty Seven", PositionGroup: "LB", BirthdateUnix: birthdate(t, "1999-06-01")},
		{ID: "turns-28-tomorrow", Name: "Turns Tomorrow", PositionGroup: "DB", BirthdateUnix: birthdate(t, "1998-11-22")},
		{ID: "turns-28-today", Name: "Turns Today", PositionGroup: "DL", BirthdateUnix: birthdate(t, "1998-11-21")},
		{ID: "age-31", Name: "Thirty One", PositionGroup: "LB", BirthdateUnix: birthdate(t, "1995-01-15")},
		{ID: "unknown", Name: "No Birthdate", PositionGroup: "DB"},
	}
	var observations []Observation
	for _, player := range players {
		observations = append(observations, risingObservations(player.ID)...)
	}

	tests := []struct {
		name         string
		config       Config
		wantIDs      []string
		wantExcluded int
		wantAges     map[string]int // players whose age should be known
	}{
		{
			name:     "no filter and no date keeps everyone without ages",
			config:   Config{},
			wantIDs:  []string{"age-27", "turns-28-tomorrow", "turns-28-today", "age-31", "unknown"},
			wantAges: map[string]int{},
		},
		{
			name:     "date without filter reports ages",
			config:   Config{AgeAsOf: "2026-11-21"},
			wantIDs:  []string{"age-27", "turns-28-tomorrow", "turns-28-today", "age-31", "unknown"},
			wantAges: map[string]int{"age-27": 27, "turns-28-tomorrow": 27, "turns-28-today": 28, "age-31": 31},
		},
		{
			name:         "max age 28 keeps players under 28 and unknown ages",
			config:       Config{MaxAge: 28, AgeAsOf: "2026-11-21"},
			wantIDs:      []string{"age-27", "turns-28-tomorrow", "unknown"},
			wantExcluded: 2,
			wantAges:     map[string]int{"age-27": 27, "turns-28-tomorrow": 27},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report, err := Analyze(Input{Players: players, Observations: observations, Config: test.config})
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]*int{}
			for _, trend := range report.Trends {
				got[trend.PlayerID] = trend.Age
			}
			if len(got) != len(test.wantIDs) {
				t.Fatalf("trend players = %v, want %v", keys(got), test.wantIDs)
			}
			for _, id := range test.wantIDs {
				age, ok := got[id]
				if !ok {
					t.Fatalf("missing %s; got %v", id, keys(got))
				}
				want, known := test.wantAges[id]
				switch {
				case known && (age == nil || *age != want):
					t.Fatalf("%s age = %v, want %d", id, age, want)
				case !known && age != nil:
					t.Fatalf("%s age = %d, want unknown", id, *age)
				}
			}
			if report.Summary.ExcludedByAge != test.wantExcluded {
				t.Fatalf("excluded_by_age = %d, want %d", report.Summary.ExcludedByAge, test.wantExcluded)
			}
			if report.Summary.Rising != len(test.wantIDs) {
				t.Fatalf("rising = %d, want %d", report.Summary.Rising, len(test.wantIDs))
			}
		})
	}
}

func TestAnalyzeAgeConfigValidation(t *testing.T) {
	players := []Player{{ID: "p", Name: "Player", PositionGroup: "LB"}}
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{"max age needs a date", Config{MaxAge: 28}, "age_as_of is required"},
		{"date must be YYYY-MM-DD", Config{MaxAge: 28, AgeAsOf: "11/21/2026"}, "age_as_of must be"},
		{"negative max age", Config{MaxAge: -1, AgeAsOf: "2026-11-21"}, "max_age must be"},
		{"implausible max age", Config{MaxAge: 61, AgeAsOf: "2026-11-21"}, "max_age must be"},
		{"bad date without filter", Config{AgeAsOf: "2026-13-01"}, "age_as_of must be"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Analyze(Input{Players: players, Config: test.config})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, test.wantErr)
			}
		})
	}
}

func keys(values map[string]*int) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}
