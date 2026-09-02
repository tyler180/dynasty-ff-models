package usage

import (
	"strings"
	"testing"
)

func TestAnalyzeDetectsSustainedRisingUsage(t *testing.T) {
	input := testInput([]int{12, 12, 12, 36, 42, 48}, 60)
	report, err := Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	trend := report.Trends[0]
	if trend.Signal != SignalRising || trend.Confidence != "high" {
		t.Fatalf("trend = %+v", trend)
	}
	if trend.Baseline.DefenseSnapShare != 0.2 || trend.Recent.DefenseSnapShare != 0.7 || trend.ChangePercentagePoints != 50 {
		t.Fatalf("windows = %+v / %+v, change = %v", trend.Baseline, trend.Recent, trend.ChangePercentagePoints)
	}
	if trend.ConfirmingGames != 3 || trend.UsageTier != "major_rotation" || report.Summary.Rising != 1 {
		t.Fatalf("trend/report = %+v / %+v", trend, report.Summary)
	}
}

func TestAnalyzeMarksSingleGameSpikeVolatile(t *testing.T) {
	input := testInput([]int{12, 12, 12, 12, 12, 60}, 60)
	report, err := Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	trend := report.Trends[0]
	if trend.Signal != SignalVolatile || trend.ConfirmingGames != 1 || trend.Confidence != "low" {
		t.Fatalf("trend = %+v", trend)
	}
}

func TestAnalyzeDetectsFallingAndStableUsage(t *testing.T) {
	falling, err := Analyze(testInput([]int{54, 54, 54, 30, 24, 18}, 60))
	if err != nil {
		t.Fatal(err)
	}
	if falling.Trends[0].Signal != SignalFalling || falling.Trends[0].ChangePercentagePoints != -50 {
		t.Fatalf("falling trend = %+v", falling.Trends[0])
	}
	stable, err := Analyze(testInput([]int{30, 30, 30, 32, 34, 33}, 60))
	if err != nil {
		t.Fatal(err)
	}
	if stable.Trends[0].Signal != SignalStable || stable.Trends[0].ChangePercentagePoints != 5 {
		t.Fatalf("stable trend = %+v", stable.Trends[0])
	}
}

func TestAnalyzeRequiresFullWindows(t *testing.T) {
	input := testInput([]int{12, 18, 24, 30, 36}, 60)
	report, err := Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	trend := report.Trends[0]
	if trend.Signal != SignalInsufficientData || trend.Confidence != "none" || !strings.Contains(trend.Reason, "needs 6 games") {
		t.Fatalf("trend = %+v", trend)
	}
}

func TestAnalyzeWeightsWindowsByTeamDefensiveSnaps(t *testing.T) {
	input := Input{
		Players: []Player{{ID: "player-1", Name: "Defender", PositionGroup: "LB"}},
		Observations: []Observation{
			{PlayerID: "player-1", GameID: "game-1", Season: 2025, Week: 1, GameType: "REG", DefenseSnaps: 20, TeamDefenseSnaps: 40, DefenseSnapPct: 0.50},
			{PlayerID: "player-1", GameID: "game-2", Season: 2025, Week: 2, GameType: "REG", DefenseSnaps: 40, TeamDefenseSnaps: 80, DefenseSnapPct: 0.50},
			{PlayerID: "player-1", GameID: "game-3", Season: 2025, Week: 3, GameType: "REG", DefenseSnaps: 40, TeamDefenseSnaps: 40, DefenseSnapPct: 1.00},
			{PlayerID: "player-1", GameID: "game-4", Season: 2025, Week: 4, GameType: "REG", DefenseSnaps: 80, TeamDefenseSnaps: 160, DefenseSnapPct: 0.50},
		},
		Config: Config{BaselineGames: 2, RecentGames: 2, MinimumConfirmingGames: 1},
	}
	report, err := Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	trend := report.Trends[0]
	if trend.Baseline.DefenseSnapShare != 0.5 || trend.Recent.DefenseSnapShare != 0.6 {
		t.Fatalf("weighted windows = %+v / %+v", trend.Baseline, trend.Recent)
	}
}

func TestAnalyzeExcludesPostseasonByDefault(t *testing.T) {
	input := testInput([]int{12, 12, 12, 36, 36, 36}, 60)
	input.Observations = append(input.Observations, Observation{
		PlayerID: "player-1", GameID: "postseason", Season: 2025, Week: 19, GameType: "POST",
		DefenseSnaps: 0, TeamDefenseSnaps: 60, DefenseSnapPct: 0,
	})
	report, err := Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Trends[0].LatestWeek != 6 || len(report.Trends[0].Weekly) != 6 {
		t.Fatalf("trend = %+v", report.Trends[0])
	}
}

func TestAnalyzeUsesSourcePercentageForWeeklyUsageAndConfirmation(t *testing.T) {
	input := testInput([]int{12, 12, 12, 18, 18, 18}, 60)
	for index := 3; index < 6; index++ {
		// Deliberately differs from 18/60 (30%) to prove that the source
		// percentage, rather than a reconstructed value, drives confirmation.
		input.Observations[index].DefenseSnapPct = 0.50
	}
	report, err := Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	trend := report.Trends[0]
	if trend.Signal != SignalRising || trend.ConfirmingGames != 3 {
		t.Fatalf("trend = %+v", trend)
	}
	if got := trend.Weekly[3].DefenseSnapShare; got != 0.50 {
		t.Fatalf("weekly source snap share = %v, want 0.50", got)
	}
	if trend.Recent.DefenseSnapShare != 0.30 {
		t.Fatalf("weighted recent snap share = %v, want 0.30", trend.Recent.DefenseSnapShare)
	}
}

func TestAnalyzeRequiresSourcePercentageWhenPlayerHasDefensiveSnaps(t *testing.T) {
	input := testInput([]int{12, 12, 12, 36, 36, 36}, 60)
	input.Observations[0].DefenseSnapPct = 0
	_, err := Analyze(input)
	if err == nil || !strings.Contains(err.Error(), "source defensive snap percentage") {
		t.Fatalf("error = %v", err)
	}
}

func TestAnalyzeRejectsDuplicateGameObservations(t *testing.T) {
	input := testInput([]int{12, 12, 12, 36, 36, 36}, 60)
	input.Observations = append(input.Observations, input.Observations[0])
	_, err := Analyze(input)
	if err == nil || !strings.Contains(err.Error(), "duplicate observation") {
		t.Fatalf("error = %v", err)
	}
}

func testInput(playerSnaps []int, teamSnaps int) Input {
	input := Input{Players: []Player{{ID: "player-1", Name: "Defender", Position: "ILB", PositionGroup: "LB"}}}
	for index, snaps := range playerSnaps {
		input.Observations = append(input.Observations, Observation{
			PlayerID: "player-1", GameID: "game-" + string(rune('a'+index)), Season: 2025, Week: index + 1,
			GameType: "REG", PositionGroup: "LB", DefenseSnaps: snaps, TeamDefenseSnaps: teamSnaps,
			DefenseSnapPct: float64(snaps) / float64(teamSnaps),
		})
	}
	return input
}
