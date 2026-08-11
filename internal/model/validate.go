package model

import (
	"errors"
	"fmt"
	"strings"
)

func (in *Input) ApplyDefaults() {
	if in.League.DiscountRate == 0 {
		in.League.DiscountRate = 0.90
	}
	if in.League.BenchWeight == 0 {
		in.League.BenchWeight = 0.15
	}
	if in.League.MaxDrops == 0 {
		in.League.MaxDrops = 5
	}
}

func (in Input) Validate() error {
	if in.League.CurrentYear <= 0 {
		return errors.New("league.current_year must be positive")
	}
	if in.League.DiscountRate <= 0 || in.League.DiscountRate > 1 {
		return errors.New("league.discount_rate must be in (0, 1]")
	}
	if in.League.BenchWeight < 0 || in.League.BenchWeight > 1 {
		return errors.New("league.bench_weight must be in [0, 1]")
	}
	if in.League.SalaryPenalty < 0 {
		return errors.New("league.salary_penalty cannot be negative")
	}
	if in.League.MaxDrops < 0 {
		return errors.New("league.max_drops cannot be negative")
	}
	if len(in.League.Seasons) == 0 {
		return errors.New("league.seasons cannot be empty")
	}
	if len(in.AvailableRookies) == 0 {
		return errors.New("available_rookies cannot be empty")
	}
	if len(in.RemainingDraftPicks) == 0 {
		return errors.New("remaining_draft_picks cannot be empty")
	}

	seasonYears := make(map[int]bool)
	for i, season := range in.League.Seasons {
		if season.Year < in.League.CurrentYear {
			return fmt.Errorf("league.seasons[%d].year is before current_year", i)
		}
		if seasonYears[season.Year] {
			return fmt.Errorf("duplicate season year %d", season.Year)
		}
		seasonYears[season.Year] = true
		if season.SalaryCap < 0 || season.MaxRoster <= 0 {
			return fmt.Errorf("season %d must have a nonnegative salary_cap and positive max_roster", season.Year)
		}
		for _, slot := range season.StartingSlots {
			if strings.TrimSpace(slot.Name) == "" || slot.Count <= 0 || len(slot.EligiblePositions) == 0 {
				return fmt.Errorf("season %d has an invalid starting slot", season.Year)
			}
		}
	}

	ids := make(map[string]string)
	for group, players := range map[string][]Player{"roster": in.Roster, "available_rookies": in.AvailableRookies} {
		for i, player := range players {
			if strings.TrimSpace(player.ID) == "" || strings.TrimSpace(player.Name) == "" || len(player.Positions) == 0 {
				return fmt.Errorf("%s[%d] must have id, name, and positions", group, i)
			}
			if previous, ok := ids[player.ID]; ok {
				return fmt.Errorf("duplicate player id %q in %s and %s", player.ID, previous, group)
			}
			ids[player.ID] = group
		}
	}
	for i, pick := range in.RemainingDraftPicks {
		if strings.TrimSpace(pick.ID) == "" || pick.Year < in.League.CurrentYear {
			return fmt.Errorf("remaining_draft_picks[%d] must have an id and a year at or after current_year", i)
		}
	}
	return nil
}
