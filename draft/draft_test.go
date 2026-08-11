package draft_test

import (
	"testing"

	"github.com/tylermclean/dynasty-ff-draft-model/draft"
)

func TestPublicRecommendAPI(t *testing.T) {
	input := draft.Input{
		League: draft.League{
			CurrentYear:  2026,
			DiscountRate: 1,
			BenchWeight:  0.15,
			MaxDrops:     1,
			Seasons: []draft.Season{{
				Year:      2026,
				SalaryCap: 100,
				MaxRoster: 2,
				StartingSlots: []draft.LineupSlot{{
					Name: "QB", Count: 1, EligiblePositions: []string{"QB"},
				}},
			}},
		},
		AvailableRookies: []draft.Player{{
			ID: "rookie-1", Name: "Rookie QB", Positions: []string{"QB"},
			ProjectedPoints: map[int]float64{2026: 200},
		}},
		RemainingDraftPicks: []draft.DraftPick{{
			ID: "1.01", Year: 2026, Salary: map[int]float64{2026: 10},
		}},
	}

	recommendation, err := draft.Recommend(input)
	if err != nil {
		t.Fatal(err)
	}
	if recommendation.SelectedPlayerID != "rookie-1" || recommendation.PickID != "1.01" {
		t.Fatalf("recommendation = %+v", recommendation)
	}
}
