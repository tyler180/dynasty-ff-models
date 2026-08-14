package analysis_test

import (
	"testing"

	"github.com/tyler180/dynasty-ff-models/analysis"
)

func TestPublicAnalysisAPI(t *testing.T) {
	snapshot := analysis.Snapshot{
		SnapshotDate: "2026-08-13",
		League: analysis.League{
			Name: "League", SalaryCap: 250, ActiveRosterLimit: 26,
			InjuredReserveLimit: 3, TaxiSquadLimit: 4,
		},
		Franchise: analysis.Franchise{Name: "Team", TotalCapHit: 240},
		Roster:    []analysis.Player{{ID: "player-1", Name: "Player", Status: "ROSTER", Salary: 10}},
	}

	result := analysis.AnalyzeWithOptions(snapshot, analysis.AnalysisOptions{})
	if result.Cap.Space != 10 || result.Roster.Active.Used != 1 {
		t.Fatalf("analysis = %+v", result)
	}
}
