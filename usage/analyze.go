package usage

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func Analyze(input Input) (Report, error) {
	input.Config.applyDefaults()
	if err := input.validate(); err != nil {
		return Report{}, err
	}
	byPlayer := make(map[string][]Observation, len(input.Players))
	for _, observation := range input.Observations {
		if !input.Config.IncludePostseason && !isRegularSeason(observation.GameType) {
			continue
		}
		byPlayer[observation.PlayerID] = append(byPlayer[observation.PlayerID], observation)
	}
	report := Report{
		Method: "Compare raw-snap-weighted recent and baseline windows; require the change to appear in multiple recent games.",
		Config: input.Config,
		Trends: make([]PlayerTrend, 0, len(input.Players)),
	}
	for _, player := range input.Players {
		observations := byPlayer[player.ID]
		sort.Slice(observations, func(i, j int) bool {
			if observations[i].Season != observations[j].Season {
				return observations[i].Season < observations[j].Season
			}
			if observations[i].Week != observations[j].Week {
				return observations[i].Week < observations[j].Week
			}
			return observations[i].GameID < observations[j].GameID
		})
		trend := analyzePlayer(player, observations, input.Config)
		report.Trends = append(report.Trends, trend)
		incrementSummary(&report.Summary, trend.Signal)
	}
	sort.SliceStable(report.Trends, func(i, j int) bool {
		left, right := report.Trends[i], report.Trends[j]
		if signalPriority(left.Signal) != signalPriority(right.Signal) {
			return signalPriority(left.Signal) < signalPriority(right.Signal)
		}
		if left.Signal == SignalFalling {
			return left.Change < right.Change
		}
		if left.Change != right.Change {
			return left.Change > right.Change
		}
		return left.Name < right.Name
	})
	return report, nil
}

func analyzePlayer(player Player, observations []Observation, config Config) PlayerTrend {
	trend := PlayerTrend{
		PlayerID: player.ID, Name: player.Name, Position: player.Position,
		PositionGroup: normalizeGroup(player.PositionGroup), Signal: SignalInsufficientData,
		Confidence: "none", Weekly: weeklyUsage(observations),
	}
	needed := config.BaselineGames + config.RecentGames
	if len(observations) < needed {
		trend.Reason = fmt.Sprintf("needs %d games but only %d valid regular-season games are available", needed, len(observations))
		return trend
	}
	recent := observations[len(observations)-config.RecentGames:]
	baselineEnd := len(observations) - config.RecentGames
	baseline := observations[baselineEnd-config.BaselineGames : baselineEnd]
	trend.Baseline = summarizeWindow(baseline)
	trend.Recent = summarizeWindow(recent)
	latest := recent[len(recent)-1]
	trend.LatestSeason, trend.LatestWeek = latest.Season, latest.Week
	if trend.Baseline.TeamDefenseSnaps < config.MinimumTeamSnapsPerWindow || trend.Recent.TeamDefenseSnaps < config.MinimumTeamSnapsPerWindow {
		trend.Reason = fmt.Sprintf(
			"each window needs at least %d team defensive snaps; baseline has %d and recent has %d",
			config.MinimumTeamSnapsPerWindow, trend.Baseline.TeamDefenseSnaps, trend.Recent.TeamDefenseSnaps,
		)
		return trend
	}
	trend.Change = round(trend.Recent.DefenseSnapShare-trend.Baseline.DefenseSnapShare, 4)
	trend.ChangePercentagePoints = round(trend.Change*100, 1)
	trend.UsageTier = usageTier(trend.Recent.DefenseSnapShare)
	confirmationDelta := config.ChangeThreshold / 2
	if trend.Change >= config.ChangeThreshold {
		trend.ConfirmingGames = confirmingGames(recent, trend.Baseline.DefenseSnapShare, confirmationDelta, 1)
		if trend.ConfirmingGames >= config.MinimumConfirmingGames {
			trend.Signal = SignalRising
		} else {
			trend.Signal = SignalVolatile
		}
	} else if trend.Change <= -config.ChangeThreshold {
		trend.ConfirmingGames = confirmingGames(recent, trend.Baseline.DefenseSnapShare, confirmationDelta, -1)
		if trend.ConfirmingGames >= config.MinimumConfirmingGames {
			trend.Signal = SignalFalling
		} else {
			trend.Signal = SignalVolatile
		}
	} else {
		trend.Signal = SignalStable
	}
	trend.Confidence = confidence(trend, config)
	trend.Reason = reason(trend, config)
	return trend
}

func summarizeWindow(observations []Observation) UsageWindow {
	window := UsageWindow{Games: len(observations)}
	for _, observation := range observations {
		window.DefenseSnaps += observation.DefenseSnaps
		window.TeamDefenseSnaps += observation.TeamDefenseSnaps
	}
	window.DefenseSnapShare = round(float64(window.DefenseSnaps)/float64(window.TeamDefenseSnaps), 4)
	return window
}

func weeklyUsage(observations []Observation) []WeeklyUsage {
	result := make([]WeeklyUsage, 0, len(observations))
	for _, observation := range observations {
		result = append(result, WeeklyUsage{
			GameID: observation.GameID, Season: observation.Season, Week: observation.Week,
			DefenseSnaps: observation.DefenseSnaps, TeamDefenseSnaps: observation.TeamDefenseSnaps,
			DefenseSnapShare: round(float64(observation.DefenseSnaps)/float64(observation.TeamDefenseSnaps), 4),
		})
	}
	return result
}

func confirmingGames(observations []Observation, baseline, threshold float64, direction int) int {
	count := 0
	for _, observation := range observations {
		share := float64(observation.DefenseSnaps) / float64(observation.TeamDefenseSnaps)
		if direction > 0 && share >= baseline+threshold {
			count++
		}
		if direction < 0 && share <= baseline-threshold {
			count++
		}
	}
	return count
}

func confidence(trend PlayerTrend, config Config) string {
	if trend.Signal == SignalVolatile {
		return "low"
	}
	if trend.Signal == SignalStable {
		return "medium"
	}
	if math.Abs(trend.Change) >= config.StrongChangeThreshold && trend.ConfirmingGames == config.RecentGames {
		return "high"
	}
	return "medium"
}

func reason(trend PlayerTrend, config Config) string {
	switch trend.Signal {
	case SignalRising:
		return fmt.Sprintf("weighted defensive snap share increased %.1f percentage points and %d of %d recent games confirm the increase", trend.ChangePercentagePoints, trend.ConfirmingGames, config.RecentGames)
	case SignalFalling:
		return fmt.Sprintf("weighted defensive snap share decreased %.1f percentage points and %d of %d recent games confirm the decrease", math.Abs(trend.ChangePercentagePoints), trend.ConfirmingGames, config.RecentGames)
	case SignalVolatile:
		return fmt.Sprintf("window usage moved %.1f percentage points, but only %d of %d recent games confirm a sustained direction", trend.ChangePercentagePoints, trend.ConfirmingGames, config.RecentGames)
	default:
		return fmt.Sprintf("weighted defensive snap share changed %.1f percentage points, below the %.1f-point signal threshold", trend.ChangePercentagePoints, config.ChangeThreshold*100)
	}
}

func usageTier(share float64) string {
	switch {
	case share >= 0.75:
		return "full_time"
	case share >= 0.50:
		return "major_rotation"
	case share >= 0.25:
		return "rotation"
	default:
		return "limited"
	}
}

func (config *Config) applyDefaults() {
	if config.BaselineGames == 0 {
		config.BaselineGames = 3
	}
	if config.RecentGames == 0 {
		config.RecentGames = 3
	}
	if config.MinimumConfirmingGames == 0 {
		config.MinimumConfirmingGames = 2
	}
	if config.MinimumTeamSnapsPerWindow == 0 {
		config.MinimumTeamSnapsPerWindow = 100
	}
	if config.ChangeThreshold == 0 {
		config.ChangeThreshold = 0.10
	}
	if config.StrongChangeThreshold == 0 {
		config.StrongChangeThreshold = 0.20
	}
}

func (input Input) validate() error {
	config := input.Config
	if config.BaselineGames < 1 || config.BaselineGames > 8 || config.RecentGames < 1 || config.RecentGames > 8 {
		return fmt.Errorf("baseline_games and recent_games must be between 1 and 8")
	}
	if config.MinimumConfirmingGames < 1 || config.MinimumConfirmingGames > config.RecentGames {
		return fmt.Errorf("minimum_confirming_games must be between 1 and recent_games")
	}
	if config.MinimumTeamSnapsPerWindow < 1 {
		return fmt.Errorf("minimum_team_snaps_per_window must be positive")
	}
	if config.ChangeThreshold <= 0 || config.ChangeThreshold > 1 || config.StrongChangeThreshold < config.ChangeThreshold || config.StrongChangeThreshold > 1 {
		return fmt.Errorf("change thresholds must satisfy 0 < change_threshold <= strong_change_threshold <= 1")
	}
	if len(input.Players) == 0 {
		return fmt.Errorf("players cannot be empty")
	}
	players := make(map[string]struct{}, len(input.Players))
	for index, player := range input.Players {
		if strings.TrimSpace(player.ID) == "" || strings.TrimSpace(player.Name) == "" {
			return fmt.Errorf("players[%d] must have an ID and name", index)
		}
		if _, duplicate := players[player.ID]; duplicate {
			return fmt.Errorf("duplicate player ID %q", player.ID)
		}
		players[player.ID] = struct{}{}
		switch normalizeGroup(player.PositionGroup) {
		case "DL", "LB", "DB":
		default:
			return fmt.Errorf("players[%d].position_group must be DL, LB, or DB", index)
		}
	}
	seenGames := make(map[string]struct{}, len(input.Observations))
	for index, observation := range input.Observations {
		if _, ok := players[observation.PlayerID]; !ok {
			return fmt.Errorf("observations[%d] references unknown player %q", index, observation.PlayerID)
		}
		if strings.TrimSpace(observation.GameID) == "" || observation.Season < 2012 || observation.Season > 2100 || observation.Week < 1 || observation.Week > 25 {
			return fmt.Errorf("observations[%d] has an invalid game, season, or week", index)
		}
		if observation.TeamDefenseSnaps <= 0 || observation.DefenseSnaps < 0 || observation.DefenseSnaps > observation.TeamDefenseSnaps {
			return fmt.Errorf("observations[%d] must have valid player and team defensive snap totals", index)
		}
		key := observation.PlayerID + "\x00" + observation.GameID
		if _, duplicate := seenGames[key]; duplicate {
			return fmt.Errorf("duplicate observation for player %q game %q", observation.PlayerID, observation.GameID)
		}
		seenGames[key] = struct{}{}
	}
	return nil
}

func normalizeGroup(group string) string {
	return strings.ToUpper(strings.TrimSpace(group))
}

func isRegularSeason(gameType string) bool {
	gameType = strings.ToUpper(strings.TrimSpace(gameType))
	return gameType == "" || gameType == "REG"
}

func incrementSummary(summary *Summary, signal string) {
	switch signal {
	case SignalRising:
		summary.Rising++
	case SignalFalling:
		summary.Falling++
	case SignalStable:
		summary.Stable++
	case SignalVolatile:
		summary.Volatile++
	default:
		summary.InsufficientData++
	}
}

func signalPriority(signal string) int {
	switch signal {
	case SignalRising:
		return 0
	case SignalVolatile:
		return 1
	case SignalFalling:
		return 2
	case SignalStable:
		return 3
	default:
		return 4
	}
}

func round(value float64, places int) float64 {
	power := math.Pow10(places)
	return math.Round(value*power) / power
}
