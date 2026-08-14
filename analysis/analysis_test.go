package analysis

import (
	"reflect"
	"testing"
	"time"
)

func TestDropEvaluationUsesSalaryProjectionAndAge(t *testing.T) {
	snapshot := Snapshot{
		SnapshotDate: "2026-08-08",
		League:       League{Name: "Test League", SalaryCap: 100, ActiveRosterLimit: 5, InjuredReserveLimit: 1, TaxiSquadLimit: 1},
		Franchise:    Franchise{Name: "Test Team", TotalCapHit: 25},
		Roster: []Player{
			{ID: "old", Name: "Older RB", Position: "RB", Salary: 10, Status: "ROSTER"},
			{ID: "young", Name: "Younger RB", Position: "RB", Salary: 10, Status: "ROSTER"},
			{ID: "cheap", Name: "Cheap Player", Position: "WR", Salary: 5, Status: "ROSTER"},
		},
		BirthdatesUnix: map[string]int64{
			"old":   time.Date(1996, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
			"young": time.Date(2003, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
			"cheap": time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		},
		Projections: Projections{Season: 2026, Source: "test", ByPlayerID: map[string]float64{
			"old": 100, "young": 100, "cheap": 20,
		}},
		Draft: Draft{CurrentYearPicks: []Pick{{Pick: "1.01", Salary: 10}}},
	}

	analysis := AnalyzeWithOptions(snapshot, AnalysisOptions{CapReliefTarget: 8})
	drops := analysis.DropEvaluation
	if !drops.Available || len(drops.Candidates) != 3 {
		t.Fatalf("drop evaluation = %+v", drops)
	}
	if drops.BestForTarget == nil || drops.BestForTarget.PlayerID != "old" {
		t.Fatalf("best target cut = %+v, want old", drops.BestForTarget)
	}
	if drops.Candidates[0].PlayerID != "cheap" {
		t.Fatalf("top unrestricted drop = %q, want cheap", drops.Candidates[0].PlayerID)
	}
	if drops.Candidates[1].PlayerID != "old" || drops.Candidates[2].PlayerID != "young" {
		t.Fatalf("age did not rank equal-production RBs correctly: %+v", drops.Candidates)
	}
}

func TestWeightedHistoricalUsesFourSeasonsAndIgnoresMissing(t *testing.T) {
	history := HistoricalPoints{Seasons: []HistoricalSeason{
		{Season: 2021, ByPlayerID: map[string]float64{"veteran": 1000}, GamesPlayedByPlayerID: map[string]int{"veteran": 1}},
		{Season: 2023, ByPlayerID: map[string]float64{"veteran": 60}, GamesPlayedByPlayerID: map[string]int{"veteran": 12}},
		{Season: 2025, ByPlayerID: map[string]float64{"veteran": 100, "rookie": 50, "zero": 100}, GamesPlayedByPlayerID: map[string]int{"veteran": 5, "rookie": 2, "zero": 10}},
		{Season: 2022, ByPlayerID: map[string]float64{"veteran": 40}, GamesPlayedByPlayerID: map[string]int{"veteran": 8}},
		{Season: 2024, ByPlayerID: map[string]float64{"veteran": 80, "zero": 0}, GamesPlayedByPlayerID: map[string]int{"veteran": 10, "zero": 10}},
	}}

	values, seasons := weightedHistorical(history)
	if got := values["veteran"].PointsPerGame; got != 9.76 {
		t.Fatalf("veteran weighted PPG = %.2f, want 9.76", got)
	}
	if got := values["rookie"].PointsPerGame; got != 25 {
		t.Fatalf("rookie weighted PPG = %.2f, want 25", got)
	}
	if got := values["zero"].PointsPerGame; got != 5.71 {
		t.Fatalf("recorded zero-point games were not retained: %.2f", got)
	}
	if !reflect.DeepEqual(seasons, []int{2025, 2024, 2023, 2022}) {
		t.Fatalf("seasons = %v", seasons)
	}
}
