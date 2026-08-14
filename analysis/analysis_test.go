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

func TestAnalyzeRanksAvailableRookiesByMarketValue(t *testing.T) {
	snapshot := Snapshot{
		SnapshotDate: "2026-08-13",
		League:       League{Name: "League", SalaryCap: 250, ActiveRosterLimit: 26, TaxiSquadLimit: 4},
		Franchise:    Franchise{Name: "Team"},
		Roster:       []Player{{ID: "veteran", Name: "Veteran", Status: "ROSTER"}},
		RookieCandidates: []RookieCandidate{
			{ID: "rookie-2", Name: "Second", Position: "LB", RookieYear: 2026, RookieRank: 2, MarketValue: 7000, ProjectedPoints: map[int]float64{2026: 180}, Source: "FantasyPros"},
			{ID: "rookie-1", Name: "First", Position: "WR", RookieYear: 2026, RookieRank: 1, MarketValue: 9000, ProjectedPoints: map[int]float64{2026: 150}, Source: "FantasyPros"},
			{ID: "rookie-3", Name: "Unranked", Position: "DB", RookieYear: 2026, Source: "MFL"},
		},
	}
	result := Analyze(snapshot)
	if !result.RookieBoard.Available || len(result.RookieBoard.Candidates) != 3 {
		t.Fatalf("rookie board = %+v", result.RookieBoard)
	}
	if result.RookieBoard.RankedCandidates != 2 || result.RookieBoard.UnrankedCandidates != 1 {
		t.Fatalf("rookie coverage = %+v", result.RookieBoard)
	}
	if got := result.RookieBoard.Candidates[0]; got.PlayerID != "rookie-1" || got.Rank != 1 {
		t.Fatalf("first rookie = %+v", got)
	}
	if got := result.RookieBoard.Candidates[2]; got.PlayerID != "rookie-3" || got.Rank != 0 || got.Valued {
		t.Fatalf("unranked rookie = %+v", got)
	}
}

func TestHistoricalFallbackIgnoresPartialSeasonProjectionSet(t *testing.T) {
	birthDate := time.Date(1997, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	snapshot := Snapshot{
		SnapshotDate: "2026-08-13",
		League:       League{Name: "League", SalaryCap: 250, ActiveRosterLimit: 26},
		Franchise:    Franchise{Name: "Team"},
		Roster: []Player{
			{ID: "first", Name: "First", Position: "WR", Salary: 10, Status: "ROSTER"},
			{ID: "second", Name: "Second", Position: "RB", Salary: 10, Status: "ROSTER"},
		},
		BirthdatesUnix: map[string]int64{"first": birthDate, "second": birthDate},
		Projections: Projections{Season: 2026, Source: "partial", ByPlayerID: map[string]float64{
			"first": 200,
		}},
		HistoricalPoints: HistoricalPoints{Source: "MFL", Seasons: []HistoricalSeason{{
			Season:                2025,
			ByPlayerID:            map[string]float64{"first": 170, "second": 136},
			GamesPlayedByPlayerID: map[string]int{"first": 17, "second": 17},
		}}},
	}

	result := AnalyzeWithOptions(snapshot, AnalysisOptions{ProjectionFallback: "historical"})
	if got := len(result.DropEvaluation.Candidates); got != 2 {
		t.Fatalf("drop candidates = %d, want 2: %+v", got, result.DropEvaluation)
	}
	productionByPlayer := map[string]float64{}
	for _, candidate := range result.DropEvaluation.Candidates {
		productionByPlayer[candidate.PlayerID] = candidate.ProductionValue
	}
	if got := productionByPlayer["first"]; got != 10 {
		t.Fatalf("historical production = %.2f, want 10", got)
	}
	if result.DropEvaluation.ProductionMetric != "historical points per game" {
		t.Fatalf("production metric = %q", result.DropEvaluation.ProductionMetric)
	}
}
