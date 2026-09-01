package usage_test

import (
	"testing"

	"github.com/tyler180/dynasty-ff-models/usage"
)

func TestPublicUsageTrendAPI(t *testing.T) {
	input := usage.Input{
		Players: []usage.Player{{ID: "player-1", Name: "Defender", PositionGroup: "LB"}},
	}
	for week, snaps := range []int{12, 12, 12, 36, 42, 48} {
		input.Observations = append(input.Observations, usage.Observation{
			PlayerID: "player-1", GameID: string(rune('a' + week)), Season: 2025, Week: week + 1,
			GameType: "REG", DefenseSnaps: snaps, TeamDefenseSnaps: 60,
		})
	}
	report, err := usage.Analyze(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Trends) != 1 || report.Trends[0].Signal != usage.SignalRising {
		t.Fatalf("report = %+v", report)
	}
}
