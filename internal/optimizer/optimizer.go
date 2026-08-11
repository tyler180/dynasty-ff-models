package optimizer

import (
	"errors"
	"math"
	"sort"

	"github.com/tylermclean/dynasty-ff-models/internal/model"
	"github.com/tylermclean/dynasty-ff-models/internal/valuation"
)

type candidateResult struct {
	player   model.Player
	drops    []model.Player
	result   valuation.Result
	feasible bool
	reason   string
}

func Recommend(in model.Input) (model.Recommendation, error) {
	if err := in.Validate(); err != nil {
		return model.Recommendation{}, err
	}
	currentPick := in.RemainingDraftPicks[0]
	laterPicks := in.RemainingDraftPicks[1:]
	evaluator := valuation.Evaluator{Input: in}
	results := make([]candidateResult, 0, len(in.AvailableRookies))

	for _, rawCandidate := range in.AvailableRookies {
		candidate := withPickContract(rawCandidate, currentPick)
		results = append(results, optimizeDrops(in, evaluator, candidate, laterPicks))
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].feasible != results[j].feasible {
			return results[i].feasible
		}
		if math.Abs(results[i].result.Score-results[j].result.Score) > 1e-9 {
			return results[i].result.Score > results[j].result.Score
		}
		if len(results[i].drops) != len(results[j].drops) {
			return len(results[i].drops) < len(results[j].drops)
		}
		return results[i].player.Name < results[j].player.Name
	})
	if len(results) == 0 || !results[0].feasible {
		return model.Recommendation{}, errors.New("no rookie produces a feasible roster within max_drops")
	}

	best := results[0]
	recommendation := model.Recommendation{
		PickID:           currentPick.ID,
		SelectedPlayerID: best.player.ID,
		SelectedPlayer:   best.player.Name,
		Score:            round(best.result.Score, 3),
		SeasonSummaries:  best.result.Seasons,
	}
	for i := range recommendation.SeasonSummaries {
		recommendation.SeasonSummaries[i].Salary = round(recommendation.SeasonSummaries[i].Salary, 2)
		recommendation.SeasonSummaries[i].SalaryCap = round(recommendation.SeasonSummaries[i].SalaryCap, 2)
		recommendation.SeasonSummaries[i].ProjectedScore = round(recommendation.SeasonSummaries[i].ProjectedScore, 3)
	}
	recommendation.DropPlayerIDs, recommendation.DropPlayers = playerLabels(best.drops)

	for i, result := range results {
		dropIDs, dropNames := playerLabels(result.drops)
		ranking := model.CandidateRanking{
			Rank:          i + 1,
			PlayerID:      result.player.ID,
			Player:        result.player.Name,
			Score:         round(result.result.Score, 3),
			DropPlayerIDs: dropIDs,
			DropPlayers:   dropNames,
			Feasible:      result.feasible,
			Reason:        result.reason,
		}
		if result.feasible {
			ranking.ScoreBehind = round(best.result.Score-result.result.Score, 3)
		}
		recommendation.Alternatives = append(recommendation.Alternatives, ranking)
	}
	return recommendation, nil
}

func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}

func withPickContract(candidate model.Player, pick model.DraftPick) model.Player {
	copy := candidate
	copy.RosterFrom = pick.Year
	copy.Salary = make(map[int]float64, len(pick.Salary)+len(candidate.Salary))
	for year, salary := range pick.Salary {
		copy.Salary[year] = salary
	}
	for year, salary := range candidate.Salary {
		copy.Salary[year] = salary
	}
	return copy
}

func optimizeDrops(in model.Input, evaluator valuation.Evaluator, candidate model.Player, laterPicks []model.DraftPick) candidateResult {
	best := candidateResult{player: candidate, result: valuation.Result{Score: math.Inf(-1)}}
	droppable := make([]int, 0, len(in.Roster))
	for index, player := range in.Roster {
		if player.CanDrop() {
			droppable = append(droppable, index)
		}
	}
	maxDrops := min(in.League.MaxDrops, len(droppable))

	for dropCount := 0; dropCount <= maxDrops; dropCount++ {
		combinations(len(droppable), dropCount, func(selected []int) {
			dropIndexes := make(map[int]bool, len(selected))
			for _, selectedIndex := range selected {
				dropIndexes[droppable[selectedIndex]] = true
			}
			players := make([]model.Player, 0, len(in.Roster)+1-len(selected))
			drops := make([]model.Player, 0, len(selected))
			for index, player := range in.Roster {
				if dropIndexes[index] {
					drops = append(drops, player)
				} else {
					players = append(players, player)
				}
			}
			players = append(players, candidate)
			value, feasible, reason := evaluator.Evaluate(players, laterPicks)
			if !feasible {
				if best.reason == "" {
					best.reason = reason
				}
				return
			}
			if !best.feasible || value.Score > best.result.Score+1e-9 || (math.Abs(value.Score-best.result.Score) <= 1e-9 && len(drops) < len(best.drops)) {
				best.feasible = true
				best.result = value
				best.drops = drops
				best.reason = ""
			}
		})
	}
	if !best.feasible {
		best.reason = "no feasible roster found within max_drops"
	}
	return best
}

func combinations(n, choose int, visit func([]int)) {
	selected := make([]int, 0, choose)
	var walk func(start int)
	walk = func(start int) {
		if len(selected) == choose {
			visit(selected)
			return
		}
		needed := choose - len(selected)
		for i := start; i <= n-needed; i++ {
			selected = append(selected, i)
			walk(i + 1)
			selected = selected[:len(selected)-1]
		}
	}
	walk(0)
}

func playerLabels(players []model.Player) ([]string, []string) {
	ids := make([]string, 0, len(players))
	names := make([]string, 0, len(players))
	for _, player := range players {
		ids = append(ids, player.ID)
		names = append(names, player.Name)
	}
	sort.Strings(ids)
	sort.Strings(names)
	return ids, names
}
