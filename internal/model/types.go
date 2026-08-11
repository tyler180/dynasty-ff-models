package model

type Input struct {
	League              League            `json:"league"`
	Roster              []Player          `json:"roster"`
	AvailableRookies    []Player          `json:"available_rookies"`
	RemainingDraftPicks []DraftPick       `json:"remaining_draft_picks"`
	EligibilityRules    []EligibilityRule `json:"eligibility_rules,omitempty"`
}

type League struct {
	CurrentYear   int      `json:"current_year"`
	DiscountRate  float64  `json:"discount_rate"`
	BenchWeight   float64  `json:"bench_weight"`
	SalaryPenalty float64  `json:"salary_penalty"`
	MaxDrops      int      `json:"max_drops"`
	Seasons       []Season `json:"seasons"`
}

type Season struct {
	Year          int            `json:"year"`
	SalaryCap     float64        `json:"salary_cap"`
	MaxRoster     int            `json:"max_roster"`
	MaxByPosition map[string]int `json:"max_by_position,omitempty"`
	StartingSlots []LineupSlot   `json:"starting_slots"`
}

type LineupSlot struct {
	Name              string   `json:"name"`
	Count             int      `json:"count"`
	EligiblePositions []string `json:"eligible_positions"`
}

type Player struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Positions       []string        `json:"positions"`
	ProjectedPoints map[int]float64 `json:"projected_points"`
	Salary          map[int]float64 `json:"salary,omitempty"`
	RosterFrom      int             `json:"roster_from,omitempty"`
	ContractThrough int             `json:"contract_through,omitempty"`
	Droppable       *bool           `json:"droppable,omitempty"`
}

func (p Player) CanDrop() bool {
	return p.Droppable == nil || *p.Droppable
}

func (p Player) Active(year int) bool {
	return (p.RosterFrom == 0 || year >= p.RosterFrom) && (p.ContractThrough == 0 || year <= p.ContractThrough)
}

type DraftPick struct {
	ID                string          `json:"id"`
	Year              int             `json:"year"`
	Salary            map[int]float64 `json:"salary"`
	ReserveRosterSpot *bool           `json:"reserve_roster_spot,omitempty"`
}

func (p DraftPick) ReservesRosterSpot() bool {
	return p.ReserveRosterSpot == nil || *p.ReserveRosterSpot
}

type EligibilityRule struct {
	EffectiveYear int      `json:"effective_year"`
	ThroughYear   int      `json:"through_year,omitempty"`
	FromPosition  string   `json:"from_position"`
	AddPositions  []string `json:"add_positions"`
}

type Recommendation struct {
	PickID           string             `json:"pick_id"`
	SelectedPlayerID string             `json:"selected_player_id"`
	SelectedPlayer   string             `json:"selected_player"`
	DropPlayerIDs    []string           `json:"drop_player_ids"`
	DropPlayers      []string           `json:"drop_players"`
	Score            float64            `json:"score"`
	SeasonSummaries  []SeasonSummary    `json:"season_summaries"`
	Alternatives     []CandidateRanking `json:"alternatives"`
}

type CandidateRanking struct {
	Rank          int      `json:"rank"`
	PlayerID      string   `json:"player_id"`
	Player        string   `json:"player"`
	Score         float64  `json:"score"`
	ScoreBehind   float64  `json:"score_behind"`
	DropPlayerIDs []string `json:"drop_player_ids"`
	DropPlayers   []string `json:"drop_players"`
	Feasible      bool     `json:"feasible"`
	Reason        string   `json:"reason,omitempty"`
}

type SeasonSummary struct {
	Year           int      `json:"year"`
	Rostered       int      `json:"rostered"`
	MaxRoster      int      `json:"max_roster"`
	Salary         float64  `json:"salary"`
	SalaryCap      float64  `json:"salary_cap"`
	ProjectedScore float64  `json:"projected_score"`
	Starters       []string `json:"starters"`
}
