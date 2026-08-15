package analysis

import (
	"encoding/json"
	"reflect"
	"strings"
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

func TestDropEvaluationProtectsMarketAssetsAndRecommendsCutPackage(t *testing.T) {
	birthDate := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	snapshot := Snapshot{
		SnapshotDate: "2026-08-15",
		League:       League{Name: "League"},
		Franchise:    Franchise{Name: "Team"},
		Roster: []Player{
			{ID: "star", Name: "Dynasty Star", Position: "WR", Salary: 25, Status: "ROSTER", DynastyRank: 12, MarketValue: 8000, MarketSource: "FantasyPros"},
			{ID: "replaceable-a", Name: "Replaceable A", Position: "WR", Salary: 5, Status: "ROSTER"},
			{ID: "replaceable-b", Name: "Replaceable B", Position: "WR", Salary: 5, Status: "ROSTER"},
		},
		BirthdatesUnix: map[string]int64{"star": birthDate, "replaceable-a": birthDate, "replaceable-b": birthDate},
		HistoricalPoints: HistoricalPoints{Seasons: []HistoricalSeason{{
			Season:                2025,
			ByPlayerID:            map[string]float64{"star": 190, "replaceable-a": 80, "replaceable-b": 70},
			GamesPlayedByPlayerID: map[string]int{"star": 16, "replaceable-a": 16, "replaceable-b": 16},
		}}},
		ReplacementLevels: ReplacementLevels{PointsPerGameByPosition: map[string]float64{"WR": 11.25}},
	}

	result := AnalyzeWithOptions(snapshot, AnalysisOptions{CapReliefTarget: 10, ProjectionFallback: "historical"})
	drops := result.DropEvaluation
	if drops.Candidates[2].PlayerID != "star" || drops.Candidates[2].Disposition != "trade_first" {
		t.Fatalf("market asset was not protected: %+v", drops.Candidates)
	}
	if !drops.TargetMet || drops.RecommendedRelief != 10 || len(drops.RecommendedCuts) != 2 {
		t.Fatalf("recommended package = %+v", drops)
	}
	if drops.RecommendedCuts[0].PlayerID != "replaceable-b" || drops.RecommendedCuts[1].PlayerID != "replaceable-a" {
		t.Fatalf("recommended cuts = %+v", drops.RecommendedCuts)
	}
	if drops.BestForTarget == nil || drops.BestForTarget.PlayerID != "replaceable-b" {
		t.Fatalf("backward-compatible best cut = %+v", drops.BestForTarget)
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
			{ID: "idp-2", Name: "Second IDP", Position: "LB", RookieYear: 2026, RookieRank: 2, MarketValue: 7000, ProjectedPoints: map[int]float64{2026: 180}, Source: "FantasyPros"},
			{ID: "offense-1", Name: "First Offense", Position: "WR", RookieYear: 2026, RookieRank: 1, MarketValue: 9000, ProjectedPoints: map[int]float64{2026: 150}, Source: "FantasyPros"},
			{ID: "offense-adp", Name: "ADP Offense", Position: "RB", RookieYear: 2026, RookieADP: 25.5, Source: "MFL rookie-only ADP"},
			{ID: "idp-1", Name: "First IDP", Position: "DE", RookieYear: 2026, RookieRank: 1, MarketValue: 8000, Source: "FantasyPros"},
			{ID: "idp-unranked", Name: "Unranked IDP", Position: "DB", RookieYear: 2026, Source: "MFL"},
			{ID: "other-unranked", Name: "Unclassified", Position: "LS", RookieYear: 2026, Source: "MFL"},
		},
	}
	result := Analyze(snapshot)
	if !result.RookieBoard.Available || result.RookieBoard.Other == nil {
		t.Fatalf("rookie board = %+v", result.RookieBoard)
	}
	if result.RookieBoard.RankedCandidates != 4 || result.RookieBoard.UnrankedCandidates != 2 {
		t.Fatalf("rookie coverage = %+v", result.RookieBoard)
	}
	if got := result.RookieBoard.Offense.Candidates[0]; got.PlayerID != "offense-1" || got.Rank != 1 {
		t.Fatalf("first offensive rookie = %+v", got)
	}
	if got := result.RookieBoard.Offense.Candidates[1]; got.PlayerID != "offense-adp" || got.Rank != 2 || got.RookieADP != 25.5 {
		t.Fatalf("ADP-ranked offensive rookie = %+v", got)
	}
	if got := result.RookieBoard.IDP.Candidates[0]; got.PlayerID != "idp-1" || got.Rank != 1 {
		t.Fatalf("first IDP rookie = %+v", got)
	}
	if got := result.RookieBoard.IDP.Candidates[2]; got.PlayerID != "idp-unranked" || got.Rank != 0 || got.Valued {
		t.Fatalf("unranked IDP rookie = %+v", got)
	}
	if got := result.RookieBoard.Other.Candidates[0]; got.PlayerID != "other-unranked" {
		t.Fatalf("other rookie = %+v", got)
	}
	if !strings.Contains(result.Warnings[0], "offense has 2 ranked and 0 unranked; IDP has 2 ranked and 1 unranked") {
		t.Fatalf("rookie coverage warning = %q", result.Warnings[0])
	}
	payload, err := json.Marshal(result.RookieBoard)
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(payload, &shape); err != nil {
		t.Fatal(err)
	}
	if _, found := shape["candidates"]; found {
		t.Fatalf("combined candidate list must not be serialized: %s", payload)
	}
	if _, found := shape["offense"]; !found {
		t.Fatalf("offense board missing from JSON: %s", payload)
	}
	if _, found := shape["idp"]; !found {
		t.Fatalf("IDP board missing from JSON: %s", payload)
	}
	report := FormatText(result)
	for _, expected := range []string{"ROOKIE BOARDS", "OFFENSE (2 ranked, 0 unranked)", "IDP (2 ranked, 1 unranked)", "#1 First Offense", "#2 ADP Offense", "rookie ADP 25.50", "#1 First IDP"} {
		if !strings.Contains(report, expected) {
			t.Errorf("formatted report is missing %q:\n%s", expected, report)
		}
	}
}

func TestAnalyzeBlendsRookieECRAndBoardRelativeADP(t *testing.T) {
	snapshot := Snapshot{
		SnapshotDate: "2026-08-13",
		League:       League{Name: "League"},
		Franchise:    Franchise{Name: "Team"},
		RookieCandidates: []RookieCandidate{
			{ID: "golday", Name: "Jake Golday", Position: "LB", RookieRank: 10, RookieADP: 40, MarketValue: 8000},
			{ID: "downs", Name: "Caleb Downs", Position: "S", RookieADP: 20},
			{ID: "styles", Name: "Sonny Styles", Position: "LB", RookieRank: 1, RookieADP: 10, MarketValue: 9000},
		},
	}

	result := Analyze(snapshot)
	got := result.RookieBoard.IDP.Candidates
	if got[0].PlayerID != "styles" || got[1].PlayerID != "downs" || got[2].PlayerID != "golday" {
		t.Fatalf("blended IDP order = %s, %s, %s", got[0].PlayerID, got[1].PlayerID, got[2].PlayerID)
	}
	if got[0].ConsensusRankScore != 1 || got[1].ConsensusRankScore != 2 || got[2].ConsensusRankScore != 7.2 {
		t.Fatalf("consensus scores = %.2f, %.2f, %.2f", got[0].ConsensusRankScore, got[1].ConsensusRankScore, got[2].ConsensusRankScore)
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
