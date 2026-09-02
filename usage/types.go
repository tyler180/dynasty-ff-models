// Package usage detects sustained changes in defensive snap participation.
package usage

const (
	SignalRising           = "rising"
	SignalFalling          = "falling"
	SignalStable           = "stable"
	SignalVolatile         = "volatile"
	SignalInsufficientData = "insufficient_data"
)

type Input struct {
	Players      []Player      `json:"players"`
	Observations []Observation `json:"observations"`
	Config       Config        `json:"config,omitempty"`
}

type Player struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Position      string `json:"position,omitempty"`
	PositionGroup string `json:"position_group"`
}

// Observation is one player's defensive participation in one game. The model
// consumes the source's per-game percentage for display and confirmation while
// retaining raw totals for correctly weighted multi-game windows.
type Observation struct {
	PlayerID         string  `json:"player_id"`
	GameID           string  `json:"game_id"`
	Season           int     `json:"season"`
	Week             int     `json:"week"`
	GameType         string  `json:"game_type,omitempty"`
	PositionGroup    string  `json:"position_group,omitempty"`
	DefenseSnaps     int     `json:"defense_snaps"`
	TeamDefenseSnaps int     `json:"team_defense_snaps"`
	DefenseSnapPct   float64 `json:"defense_snap_pct"`
}

type Config struct {
	BaselineGames             int     `json:"baseline_games,omitempty"`
	RecentGames               int     `json:"recent_games,omitempty"`
	MinimumConfirmingGames    int     `json:"minimum_confirming_games,omitempty"`
	MinimumTeamSnapsPerWindow int     `json:"minimum_team_snaps_per_window,omitempty"`
	ChangeThreshold           float64 `json:"change_threshold,omitempty"`
	StrongChangeThreshold     float64 `json:"strong_change_threshold,omitempty"`
	IncludePostseason         bool    `json:"include_postseason,omitempty"`
}

type Report struct {
	Method  string        `json:"method"`
	Config  Config        `json:"config"`
	Trends  []PlayerTrend `json:"trends"`
	Summary Summary       `json:"summary"`
}

type Summary struct {
	Rising           int `json:"rising"`
	Falling          int `json:"falling"`
	Stable           int `json:"stable"`
	Volatile         int `json:"volatile"`
	InsufficientData int `json:"insufficient_data"`
}

type PlayerTrend struct {
	PlayerID               string        `json:"player_id"`
	Name                   string        `json:"name"`
	Position               string        `json:"position,omitempty"`
	PositionGroup          string        `json:"position_group"`
	Signal                 string        `json:"signal"`
	Confidence             string        `json:"confidence"`
	UsageTier              string        `json:"usage_tier,omitempty"`
	Baseline               UsageWindow   `json:"baseline"`
	Recent                 UsageWindow   `json:"recent"`
	Change                 float64       `json:"change"`
	ChangePercentagePoints float64       `json:"change_percentage_points"`
	ConfirmingGames        int           `json:"confirming_games"`
	LatestSeason           int           `json:"latest_season,omitempty"`
	LatestWeek             int           `json:"latest_week,omitempty"`
	Weekly                 []WeeklyUsage `json:"weekly"`
	Reason                 string        `json:"reason"`
}

type UsageWindow struct {
	Games            int     `json:"games"`
	DefenseSnaps     int     `json:"defense_snaps"`
	TeamDefenseSnaps int     `json:"team_defense_snaps"`
	DefenseSnapShare float64 `json:"defense_snap_share"`
}

type WeeklyUsage struct {
	GameID           string  `json:"game_id"`
	Season           int     `json:"season"`
	Week             int     `json:"week"`
	DefenseSnaps     int     `json:"defense_snaps"`
	TeamDefenseSnaps int     `json:"team_defense_snaps"`
	DefenseSnapShare float64 `json:"defense_snap_share"`
}
