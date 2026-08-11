package optimizer

import (
	"testing"

	"github.com/tylermclean/dynasty-ff-models/internal/model"
)

func TestFutureEligibilityChangesRecommendation(t *testing.T) {
	in := model.Input{
		League: model.League{
			CurrentYear:  2027,
			DiscountRate: 0.90,
			BenchWeight:  0.15,
			MaxDrops:     2,
			Seasons: []model.Season{
				{Year: 2027, SalaryCap: 100, MaxRoster: 2, StartingSlots: wrSlot()},
				{Year: 2028, SalaryCap: 100, MaxRoster: 2, StartingSlots: wrSlot()},
			},
		},
		Roster: []model.Player{
			{ID: "veteran-wr", Name: "Veteran WR", Positions: []string{"WR"}, ProjectedPoints: map[int]float64{2027: 50, 2028: 50}},
		},
		AvailableRookies: []model.Player{
			{ID: "future-te", Name: "Future TE", Positions: []string{"TE"}, ProjectedPoints: map[int]float64{2027: 0, 2028: 100}},
			{ID: "steady-wr", Name: "Steady WR", Positions: []string{"WR"}, ProjectedPoints: map[int]float64{2027: 60, 2028: 60}},
		},
		RemainingDraftPicks: []model.DraftPick{{ID: "2027-1.01", Year: 2027, Salary: map[int]float64{2027: 1, 2028: 1}}},
		EligibilityRules: []model.EligibilityRule{
			{EffectiveYear: 2028, FromPosition: "TE", AddPositions: []string{"WR"}},
		},
	}

	recommendation, err := Recommend(in)
	if err != nil {
		t.Fatal(err)
	}
	if recommendation.SelectedPlayerID != "future-te" {
		t.Fatalf("selected %q, want future-te", recommendation.SelectedPlayerID)
	}

	in.EligibilityRules = nil
	recommendation, err = Recommend(in)
	if err != nil {
		t.Fatal(err)
	}
	if recommendation.SelectedPlayerID != "steady-wr" {
		t.Fatalf("without eligibility rule selected %q, want steady-wr", recommendation.SelectedPlayerID)
	}
}

func TestRecommendationReservesLaterPickAndMakesCapFeasible(t *testing.T) {
	in := model.Input{
		League: model.League{
			CurrentYear:  2027,
			DiscountRate: 1,
			BenchWeight:  0.15,
			MaxDrops:     2,
			Seasons: []model.Season{
				{Year: 2027, SalaryCap: 100, MaxRoster: 3, StartingSlots: wrSlot()},
			},
		},
		Roster: []model.Player{
			{ID: "expensive", Name: "Expensive Veteran", Positions: []string{"WR"}, ProjectedPoints: map[int]float64{2027: 100}, Salary: map[int]float64{2027: 80}},
			{ID: "cheap", Name: "Cheap Bench", Positions: []string{"WR"}, ProjectedPoints: map[int]float64{2027: 20}, Salary: map[int]float64{2027: 10}},
		},
		AvailableRookies: []model.Player{
			{ID: "rookie", Name: "Rookie", Positions: []string{"WR"}, ProjectedPoints: map[int]float64{2027: 90}},
		},
		RemainingDraftPicks: []model.DraftPick{
			{ID: "2027-1.01", Year: 2027, Salary: map[int]float64{2027: 20}},
			{ID: "2027-2.01", Year: 2027, Salary: map[int]float64{2027: 5}},
		},
	}

	recommendation, err := Recommend(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(recommendation.DropPlayerIDs) != 1 || recommendation.DropPlayerIDs[0] != "expensive" {
		t.Fatalf("drops = %v, want [expensive]", recommendation.DropPlayerIDs)
	}
	season := recommendation.SeasonSummaries[0]
	if season.Rostered != 3 || season.Salary != 35 {
		t.Fatalf("season summary = rostered %d salary %.2f, want 3 and 35", season.Rostered, season.Salary)
	}
}

func wrSlot() []model.LineupSlot {
	return []model.LineupSlot{{Name: "WR", Count: 1, EligiblePositions: []string{"WR"}}}
}
