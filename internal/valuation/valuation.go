package valuation

import (
	"fmt"
	"math"
	"sort"

	"github.com/tylermclean/dynasty-ff-models/internal/model"
)

type Result struct {
	Score   float64
	Seasons []model.SeasonSummary
}

type Evaluator struct {
	Input model.Input
}

func (e Evaluator) Evaluate(players []model.Player, laterPicks []model.DraftPick) (Result, bool, string) {
	result := Result{}
	seasons := append([]model.Season(nil), e.Input.League.Seasons...)
	sort.Slice(seasons, func(i, j int) bool { return seasons[i].Year < seasons[j].Year })

	for _, season := range seasons {
		active := activePlayers(players, season.Year)
		reserved, pickSalary := reservedPicks(laterPicks, season.Year)
		rostered := len(active) + reserved
		if rostered > season.MaxRoster {
			return Result{}, false, fmt.Sprintf("%d roster size %d exceeds limit %d", season.Year, rostered, season.MaxRoster)
		}
		if !positionCapsOK(active, season, e.Input.EligibilityRules) {
			return Result{}, false, fmt.Sprintf("%d positional roster limits cannot be satisfied", season.Year)
		}

		salary := pickSalary
		for _, player := range active {
			salary += player.Salary[season.Year]
		}
		if salary > season.SalaryCap+1e-9 {
			return Result{}, false, fmt.Sprintf("%d salary %.2f exceeds cap %.2f", season.Year, salary, season.SalaryCap)
		}

		lineupScore, starters := bestLineup(active, season, e.Input.League.BenchWeight, e.Input.EligibilityRules)
		seasonScore := lineupScore - e.Input.League.SalaryPenalty*salary
		yearsAway := season.Year - e.Input.League.CurrentYear
		discount := math.Pow(e.Input.League.DiscountRate, float64(yearsAway))
		result.Score += seasonScore * discount
		result.Seasons = append(result.Seasons, model.SeasonSummary{
			Year:           season.Year,
			Rostered:       rostered,
			MaxRoster:      season.MaxRoster,
			Salary:         salary,
			SalaryCap:      season.SalaryCap,
			ProjectedScore: lineupScore,
			Starters:       starters,
		})
	}
	return result, true, ""
}

func activePlayers(players []model.Player, year int) []model.Player {
	active := make([]model.Player, 0, len(players))
	for _, player := range players {
		if player.Active(year) {
			active = append(active, player)
		}
	}
	return active
}

func reservedPicks(picks []model.DraftPick, year int) (int, float64) {
	count := 0
	salary := 0.0
	for _, pick := range picks {
		if pick.Year > year {
			continue
		}
		if pick.ReservesRosterSpot() {
			count++
		}
		salary += pick.Salary[year]
	}
	return count, salary
}

func eligiblePositions(player model.Player, year int, rules []model.EligibilityRule) map[string]bool {
	positions := make(map[string]bool, len(player.Positions))
	for _, position := range player.Positions {
		positions[position] = true
	}
	for _, rule := range rules {
		if year < rule.EffectiveYear || (rule.ThroughYear != 0 && year > rule.ThroughYear) || !positions[rule.FromPosition] {
			continue
		}
		for _, position := range rule.AddPositions {
			positions[position] = true
		}
	}
	return positions
}

func positionCapsOK(players []model.Player, season model.Season, rules []model.EligibilityRule) bool {
	if len(season.MaxByPosition) == 0 {
		return true
	}
	counts := make(map[string]int)
	var assign func(int) bool
	assign = func(index int) bool {
		if index == len(players) {
			return true
		}
		positions := eligiblePositions(players[index], season.Year, rules)
		for position := range positions {
			limit, constrained := season.MaxByPosition[position]
			if constrained && counts[position] >= limit {
				continue
			}
			counts[position]++
			if assign(index + 1) {
				return true
			}
			counts[position]--
		}
		return false
	}
	return assign(0)
}

type lineupState struct {
	score    float64
	starters []string
}

func bestLineup(players []model.Player, season model.Season, benchWeight float64, rules []model.EligibilityRule) (float64, []string) {
	stateCount := 1
	multipliers := make([]int, len(season.StartingSlots))
	for i, slot := range season.StartingSlots {
		multipliers[i] = stateCount
		stateCount *= slot.Count + 1
	}
	dp := make([]lineupState, stateCount)
	reachable := make([]bool, stateCount)
	reachable[0] = true
	baseline := 0.0

	for _, player := range players {
		points := player.ProjectedPoints[season.Year]
		baseline += benchWeight * points
		next := append([]lineupState(nil), dp...)
		nextReachable := append([]bool(nil), reachable...)
		positions := eligiblePositions(player, season.Year, rules)
		for encoded, state := range dp {
			if !reachable[encoded] {
				continue
			}
			for slotIndex, slot := range season.StartingSlots {
				if !slotEligible(slot, positions) {
					continue
				}
				occupied := (encoded / multipliers[slotIndex]) % (slot.Count + 1)
				if occupied >= slot.Count {
					continue
				}
				newEncoded := encoded + multipliers[slotIndex]
				newScore := state.score + (1-benchWeight)*points
				if !nextReachable[newEncoded] || newScore > next[newEncoded].score+1e-9 {
					names := append([]string(nil), state.starters...)
					names = append(names, player.Name+" ("+slot.Name+")")
					next[newEncoded] = lineupState{score: newScore, starters: names}
					nextReachable[newEncoded] = true
				}
			}
		}
		dp, reachable = next, nextReachable
	}

	best := lineupState{score: math.Inf(-1)}
	for encoded, state := range dp {
		if reachable[encoded] && state.score > best.score {
			best = state
		}
	}
	sort.Strings(best.starters)
	return baseline + best.score, best.starters
}

func slotEligible(slot model.LineupSlot, positions map[string]bool) bool {
	for _, position := range slot.EligiblePositions {
		if positions[position] {
			return true
		}
	}
	return false
}
