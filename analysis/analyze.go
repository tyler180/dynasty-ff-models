package analysis

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

func Analyze(snapshot Snapshot) Analysis {
	return AnalyzeWithOptions(snapshot, AnalysisOptions{})
}

func AnalyzeWithOptions(snapshot Snapshot, options AnalysisOptions) Analysis {
	year, _ := strconv.Atoi(snapshot.SnapshotDate[:4])
	active := playersWithStatus(snapshot.Roster, "ROSTER")
	ir := playersWithStatus(snapshot.Roster, "INJURED_RESERVE")
	taxi := playersWithStatus(snapshot.Roster, "TAXI_SQUAD")
	capSpace := snapshot.League.SalaryCap - snapshot.Franchise.TotalCapHit

	analysis := Analysis{
		SnapshotDate: snapshot.SnapshotDate,
		League:       snapshot.League.Name,
		Team:         snapshot.Franchise.Name,
		Cap: CapSummary{
			Used:  round(snapshot.Franchise.TotalCapHit, 2),
			Limit: round(snapshot.League.SalaryCap, 2),
			Space: round(capSpace, 2),
		},
		Roster: RosterSummary{
			Active:         slotSummary(len(active), snapshot.League.ActiveRosterLimit),
			InjuredReserve: slotSummary(len(ir), snapshot.League.InjuredReserveLimit),
			Taxi:           slotSummary(len(taxi), snapshot.League.TaxiSquadLimit),
		},
		Draft: DraftSummary{
			Status:             snapshot.Draft.Status,
			AvailabilityWindow: snapshot.Draft.AvailabilityPollWindow,
			PickCount:          len(snapshot.Draft.CurrentYearPicks),
		},
		ComplianceScenarios: []ComplianceScenario{},
		Warnings:            []string{},
	}
	analysis.TaxiCompliance.MustLeaveTaxi = []PlayerSummary{}
	analysis.TaxiCompliance.UnknownEligibility = []PlayerSummary{}

	for _, pick := range snapshot.Draft.CurrentYearPicks {
		shortfall := math.Max(0, pick.Salary-capSpace)
		analysis.Draft.Picks = append(analysis.Draft.Picks, PickAssessment{
			Pick:               pick.Pick,
			Overall:            pick.Overall,
			Salary:             pick.Salary,
			FitsActiveNow:      len(active) < snapshot.League.ActiveRosterLimit && shortfall == 0,
			ActiveCapShortfall: round(shortfall, 2),
			FitsTaxiNow:        len(taxi) < snapshot.League.TaxiSquadLimit,
		})
		analysis.Draft.TotalSalaryIfActive += pick.Salary
	}
	analysis.Draft.TotalSalaryIfActive = round(analysis.Draft.TotalSalaryIfActive, 2)

	for _, player := range taxi {
		summary := summarizePlayer(player)
		if player.RookieYear == 0 {
			analysis.TaxiCompliance.UnknownEligibility = append(analysis.TaxiCompliance.UnknownEligibility, summary)
			continue
		}
		if year-player.RookieYear >= 2 {
			analysis.TaxiCompliance.MustLeaveTaxi = append(analysis.TaxiCompliance.MustLeaveTaxi, summary)
		}
	}
	analysis.TaxiCompliance.CurrentYear = year
	analysis.TaxiCompliance.SlotsOpenedAfterMove = len(analysis.TaxiCompliance.MustLeaveTaxi)
	sort.Slice(analysis.TaxiCompliance.MustLeaveTaxi, func(i, j int) bool {
		return analysis.TaxiCompliance.MustLeaveTaxi[i].Salary > analysis.TaxiCompliance.MustLeaveTaxi[j].Salary
	})

	analysis.ComplianceScenarios = complianceScenarios(snapshot, active, taxi, analysis.TaxiCompliance.MustLeaveTaxi)
	analysis.HistoricalEfficiency = historicalEfficiency(snapshot, active)
	analysis.DropEvaluation = evaluateDrops(snapshot, options)
	analysis.RookieBoard = evaluateRookies(snapshot, year)
	analysis.Warnings = []string{
		"Historical fallback uses up to four prior seasons; replacement value and early-career protection improve cut analysis, but explicit dynasty market values are still absent.",
		"The rookie draft is voluntary; total pick salary is an upper bound only if every pick is used on an active player.",
		"Roster status and player eligibility must be refreshed again when the draft is scheduled.",
	}
	if !analysis.RookieBoard.Available {
		analysis.Warnings = append([]string{"No valued rookie candidates are present, so this report cannot rank draft selections."}, analysis.Warnings...)
	} else if analysis.RookieBoard.UnrankedCandidates > 0 {
		warning := fmt.Sprintf(
			"Rookie value coverage is partial: offense has %d ranked and %d unranked; IDP has %d ranked and %d unranked.",
			analysis.RookieBoard.Offense.RankedCandidates, analysis.RookieBoard.Offense.UnrankedCandidates,
			analysis.RookieBoard.IDP.RankedCandidates, analysis.RookieBoard.IDP.UnrankedCandidates,
		)
		if analysis.RookieBoard.Other != nil {
			warning += fmt.Sprintf(" Other positions have %d ranked and %d unranked.",
				analysis.RookieBoard.Other.RankedCandidates, analysis.RookieBoard.Other.UnrankedCandidates)
		}
		analysis.Warnings = append([]string{warning}, analysis.Warnings...)
	}
	for _, note := range snapshot.SourceReconciliation {
		if strings.HasPrefix(note, "Sync warning: ") {
			analysis.Warnings = append(analysis.Warnings, strings.TrimPrefix(note, "Sync warning: "))
		}
	}
	return analysis
}

func evaluateRookies(snapshot Snapshot, year int) RookieBoard {
	board := RookieBoard{
		Offense: RookieBoardPool{Candidates: []RookieAssessment{}},
		IDP:     RookieBoardPool{Candidates: []RookieAssessment{}},
		Caution: "Offense and IDP are ranked independently. Each board blends rookie ECR with board-relative MFL ADP, but the two boards are not comparable until league-specific IDP scarcity and scoring are calibrated.",
	}
	for _, candidate := range snapshot.RookieCandidates {
		if candidate.ID == "" || candidate.Name == "" {
			continue
		}
		valued := candidate.MarketValue > 0 || candidate.RookieRank > 0 || candidate.RookieADP > 0 || candidate.DynastyRank > 0 || candidate.ProjectedPoints[year] > 0
		assessment := RookieAssessment{
			PlayerID: candidate.ID, Name: candidate.Name, Position: candidate.Position, NFLTeam: candidate.NFLTeam,
			RookieRank: candidate.RookieRank, RookieADP: candidate.RookieADP,
			DynastyRank: candidate.DynastyRank, MarketValue: candidate.MarketValue,
			ProjectedPoints: candidate.ProjectedPoints[year], Valued: valued,
		}
		switch rookiePositionGroup(candidate.Position) {
		case "offense":
			board.Offense.Candidates = append(board.Offense.Candidates, assessment)
			setRookiePoolSource(&board.Offense, candidate.Source, valued)
		case "idp":
			board.IDP.Candidates = append(board.IDP.Candidates, assessment)
			setRookiePoolSource(&board.IDP, candidate.Source, valued)
		default:
			if board.Other == nil {
				board.Other = &RookieBoardPool{Candidates: []RookieAssessment{}}
			}
			board.Other.Candidates = append(board.Other.Candidates, assessment)
			setRookiePoolSource(board.Other, candidate.Source, valued)
		}
	}
	finalizeRookiePool(&board.Offense)
	finalizeRookiePool(&board.IDP)
	if board.Other != nil {
		finalizeRookiePool(board.Other)
	}
	board.RankedCandidates = board.Offense.RankedCandidates + board.IDP.RankedCandidates
	board.UnrankedCandidates = board.Offense.UnrankedCandidates + board.IDP.UnrankedCandidates
	if board.Other != nil {
		board.RankedCandidates += board.Other.RankedCandidates
		board.UnrankedCandidates += board.Other.UnrankedCandidates
	}
	board.Available = board.RankedCandidates > 0
	if board.Offense.Source != "" {
		board.Source = board.Offense.Source
	} else if board.IDP.Source != "" {
		board.Source = board.IDP.Source
	} else if board.Other != nil {
		board.Source = board.Other.Source
	}
	return board
}

func setRookiePoolSource(pool *RookieBoardPool, source string, valued bool) {
	if valued && pool.Source == "" {
		pool.Source = source
	}
}

func finalizeRookiePool(pool *RookieBoardPool) {
	adpRanks := boardRelativeADPRanks(pool.Candidates)
	for index := range pool.Candidates {
		ecr, adpRank := pool.Candidates[index].RookieRank, adpRanks[index]
		switch {
		case ecr > 0 && adpRank > 0:
			// ECR is the stronger signal, while ADP is strong enough to move a
			// player across the former ECR/no-ECR boundary.
			pool.Candidates[index].ConsensusRankScore = round(0.60*ecr+0.40*adpRank, 2)
		case ecr > 0:
			pool.Candidates[index].ConsensusRankScore = ecr
		case adpRank > 0:
			pool.Candidates[index].ConsensusRankScore = adpRank
		}
	}
	sort.SliceStable(pool.Candidates, func(i, j int) bool {
		if pool.Candidates[i].Valued != pool.Candidates[j].Valued {
			return pool.Candidates[i].Valued
		}
		leftScore, rightScore := pool.Candidates[i].ConsensusRankScore, pool.Candidates[j].ConsensusRankScore
		if (leftScore > 0) != (rightScore > 0) {
			return leftScore > 0
		}
		if leftScore != rightScore {
			return leftScore < rightScore
		}
		leftRank, rightRank := pool.Candidates[i].RookieRank, pool.Candidates[j].RookieRank
		if leftRank == 0 {
			leftRank = math.Inf(1)
		}
		if rightRank == 0 {
			rightRank = math.Inf(1)
		}
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		leftADP, rightADP := pool.Candidates[i].RookieADP, pool.Candidates[j].RookieADP
		if leftADP == 0 {
			leftADP = math.Inf(1)
		}
		if rightADP == 0 {
			rightADP = math.Inf(1)
		}
		if leftADP != rightADP {
			return leftADP < rightADP
		}
		if pool.Candidates[i].MarketValue != pool.Candidates[j].MarketValue {
			return pool.Candidates[i].MarketValue > pool.Candidates[j].MarketValue
		}
		if pool.Candidates[i].ProjectedPoints != pool.Candidates[j].ProjectedPoints {
			return pool.Candidates[i].ProjectedPoints > pool.Candidates[j].ProjectedPoints
		}
		return pool.Candidates[i].Name < pool.Candidates[j].Name
	})
	for index := range pool.Candidates {
		if pool.Candidates[index].Valued {
			pool.RankedCandidates++
			pool.Candidates[index].Rank = pool.RankedCandidates
		} else {
			pool.UnrankedCandidates++
		}
	}
	pool.Available = pool.RankedCandidates > 0
}

func boardRelativeADPRanks(candidates []RookieAssessment) map[int]float64 {
	indices := make([]int, 0, len(candidates))
	for index, candidate := range candidates {
		if candidate.RookieADP > 0 {
			indices = append(indices, index)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return candidates[indices[i]].RookieADP < candidates[indices[j]].RookieADP
	})
	ranks := make(map[int]float64, len(indices))
	for position := 0; position < len(indices); {
		end := position + 1
		for end < len(indices) && candidates[indices[end]].RookieADP == candidates[indices[position]].RookieADP {
			end++
		}
		// Equal ADPs share their average ordinal rank.
		rank := (float64(position+1) + float64(end)) / 2
		for cursor := position; cursor < end; cursor++ {
			ranks[indices[cursor]] = rank
		}
		position = end
	}
	return ranks
}

func rookiePositionGroup(position string) string {
	switch strings.ToUpper(strings.TrimSpace(position)) {
	case "QB", "RB", "WR", "TE", "K", "PK", "P":
		return "offense"
	case "DE", "DT", "DL", "EDGE", "LB", "ILB", "OLB", "CB", "S", "DB":
		return "idp"
	default:
		return "other"
	}
}

func evaluateDrops(snapshot Snapshot, options AnalysisOptions) DropEvaluation {
	evaluation := DropEvaluation{
		CapReliefTarget:  round(math.Max(0, options.CapReliefTarget), 2),
		Candidates:       []DropCandidate{},
		MissingPlayerIDs: []string{},
		Method:           "Classify players before ranking: protect early-career and dynasty-ranked assets, then choose the lowest-retention-cost combination of actual drop candidates that meets the cap target.",
		Caution:          "Replacement value, age, development, available dynasty ECR, and historical winning bids are modeled. Losing and pending bids are private, so acquisition salary and competition are estimates rather than guarantees.",
	}
	points := snapshot.Projections.ByPlayerID
	historyByPlayer := map[string]historicalValue{}
	evaluation.ProjectionSeason = snapshot.Projections.Season
	evaluation.ProjectionSource = snapshot.Projections.Source
	evaluation.ProductionMetric = "projected season points"
	if options.ProjectionFallback == "historical" {
		historyByPlayer, _ = weightedHistorical(snapshot.HistoricalPoints)
		points = make(map[string]float64, len(historyByPlayer))
		allSeasons := historicalSeasonList(historyByPlayer)
		for id, value := range historyByPlayer {
			points[id] = value.PointsPerGame
		}
		evaluation.ProjectionSeason = 0
		evaluation.ProjectionSource = fmt.Sprintf("%s recency-weighted historical points-per-game fallback", formatSeasons(allSeasons))
		evaluation.ProductionMetric = "historical points per game"
		evaluation.ReplacementSource = snapshot.ReplacementLevels.Source
		evaluation.Caution = "Historical PPG fallback is enabled for exploratory use. Replacement options are current MFL free agents; estimated salaries use historical winning bids because losing and pending bids are not exposed."
	}
	if len(points) == 0 {
		evaluation.ProjectionSource = "unavailable"
		for _, player := range snapshot.Roster {
			if capRelief(player) > 0 {
				evaluation.MissingPlayerIDs = append(evaluation.MissingPlayerIDs, player.ID)
			}
		}
		return evaluation
	}

	snapshotDate, _ := time.Parse("2006-01-02", snapshot.SnapshotDate)
	for _, player := range snapshot.Roster {
		relief := capRelief(player)
		if relief <= 0 {
			continue
		}
		projected, ok := points[player.ID]
		birthdate, hasBirthdate := snapshot.BirthdatesUnix[player.ID]
		if !ok || !hasBirthdate {
			evaluation.MissingPlayerIDs = append(evaluation.MissingPlayerIDs, player.ID)
			continue
		}
		age := ageOn(time.Unix(birthdate, 0).UTC(), snapshotDate)
		factor := ageFactor(player.Position, age)
		development := 1.0
		adjusted := math.Max(1, projected*factor)
		retention := adjusted + player.MarketValue/1000
		candidate := DropCandidate{
			PlayerID:              player.ID,
			Name:                  player.Name,
			Position:              player.Position,
			Age:                   age,
			SalaryCapRelief:       round(relief, 2),
			ProductionValue:       round(projected, 2),
			AgeFactor:             round(factor, 3),
			DevelopmentFactor:     1,
			AgeAdjustedProduction: round(adjusted, 2),
			RetentionValue:        round(retention, 3),
			DropScore:             round(100*relief/retention, 3),
			DynastyRank:           player.DynastyRank,
			MarketValue:           player.MarketValue,
			MarketSource:          player.MarketSource,
			Disposition:           "cap_efficiency_only",
			DispositionReason:     "No compatible replacement-level input is available for this production metric.",
		}
		if marketProtected(candidate) {
			candidate.Disposition = "trade_first"
			candidate.DispositionReason = "Protected from an outright drop by current dynasty market ranking; trade before considering release."
		}
		if historical, ok := historyByPlayer[player.ID]; ok {
			candidate.ProductionSeasons = append([]int(nil), historical.Seasons...)
			candidate.HistoricalGames = historical.Games
			candidate.CareerSeasons = len(historical.Seasons)
			if replacement, exists := snapshot.ReplacementLevels.PointsPerGameByPosition[player.Position]; exists {
				development = developmentFactor(age, candidate.CareerSeasons)
				vorp := projected - replacement
				adjustedVORP := math.Max(0, vorp) * factor * development
				candidate.DevelopmentFactor = round(development, 3)
				candidate.ReplacementPointsPerGame = round(replacement, 2)
				candidate.ValueOverReplacement = round(vorp, 2)
				candidate.DynastyAdjustedVORP = round(adjustedVORP, 2)
				productionShare := math.Max(0.1, projected/replacement)
				retention := productionShare + adjustedVORP + player.MarketValue/1000
				candidate.RetentionValue = round(retention, 3)
				candidate.DropScore = round(100*relief/retention, 3)
				candidate.Disposition, candidate.DispositionReason = disposition(candidate, vorp)
			}
		}
		candidate.ReplacementOptions = replacementOptions(snapshot, player, candidate)
		evaluation.Candidates = append(evaluation.Candidates, candidate)
	}
	sort.Strings(evaluation.MissingPlayerIDs)
	sort.Slice(evaluation.Candidates, func(i, j int) bool {
		leftPriority := dispositionPriority(evaluation.Candidates[i].Disposition)
		rightPriority := dispositionPriority(evaluation.Candidates[j].Disposition)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if evaluation.Candidates[i].DropScore != evaluation.Candidates[j].DropScore {
			return evaluation.Candidates[i].DropScore > evaluation.Candidates[j].DropScore
		}
		if evaluation.Candidates[i].ValueOverReplacement != evaluation.Candidates[j].ValueOverReplacement {
			return evaluation.Candidates[i].ValueOverReplacement < evaluation.Candidates[j].ValueOverReplacement
		}
		if evaluation.Candidates[i].SalaryCapRelief != evaluation.Candidates[j].SalaryCapRelief {
			return evaluation.Candidates[i].SalaryCapRelief > evaluation.Candidates[j].SalaryCapRelief
		}
		return evaluation.Candidates[i].Name < evaluation.Candidates[j].Name
	})
	for i := range evaluation.Candidates {
		evaluation.Candidates[i].Rank = i + 1
		switch evaluation.Candidates[i].Disposition {
		case "drop_candidate", "cap_efficiency_only":
			evaluation.DropCandidates = append(evaluation.DropCandidates, evaluation.Candidates[i])
		case "trade_first":
			evaluation.TradeFirst = append(evaluation.TradeFirst, evaluation.Candidates[i])
		case "hold_develop":
			evaluation.HoldDevelop = append(evaluation.HoldDevelop, evaluation.Candidates[i])
		}
	}
	evaluation.Available = len(evaluation.Candidates) > 0
	evaluation.RecommendedCuts, evaluation.RecommendedRelief = recommendCutPackage(evaluation.DropCandidates, evaluation.CapReliefTarget)
	evaluation.TargetMet = evaluation.CapReliefTarget <= 0 || evaluation.RecommendedRelief >= evaluation.CapReliefTarget
	if len(evaluation.RecommendedCuts) > 0 {
		best := evaluation.RecommendedCuts[0]
		evaluation.BestForTarget = &best
	}
	return evaluation
}

func replacementOptions(snapshot Snapshot, dropped Player, candidate DropCandidate) []ReplacementOption {
	pool := append([]ReplacementCandidate(nil), snapshot.ReplacementLevels.CandidatesByPosition[dropped.Position]...)
	if len(pool) == 0 {
		return nil
	}
	sort.SliceStable(pool, func(i, j int) bool {
		leftEstablished := pool[i].HistoricalGames >= snapshot.ReplacementLevels.MinimumHistoricalGames
		rightEstablished := pool[j].HistoricalGames >= snapshot.ReplacementLevels.MinimumHistoricalGames
		if leftEstablished != rightEstablished {
			return leftEstablished
		}
		if leftEstablished && pool[i].HistoricalPointsPerGame != pool[j].HistoricalPointsPerGame {
			return pool[i].HistoricalPointsPerGame > pool[j].HistoricalPointsPerGame
		}
		if pool[i].MarketValue != pool[j].MarketValue {
			return pool[i].MarketValue > pool[j].MarketValue
		}
		if pool[i].ProjectedPoints != pool[j].ProjectedPoints {
			return pool[i].ProjectedPoints > pool[j].ProjectedPoints
		}
		return pool[i].Name < pool[j].Name
	})

	activeOpen := snapshot.League.ActiveRosterLimit - countStatus(snapshot.Roster, "ROSTER")
	afterOpen := activeOpen - 1
	if strings.EqualFold(dropped.Status, "ROSTER") {
		afterOpen = activeOpen
	}
	capSpace := snapshot.League.SalaryCap - snapshot.Franchise.TotalCapHit
	result := make([]ReplacementOption, 0, 5)
	established, speculative := 0, 0
	for _, replacement := range pool {
		isEstablished := replacement.HistoricalGames >= snapshot.ReplacementLevels.MinimumHistoricalGames
		if isEstablished {
			if established >= 3 {
				continue
			}
			established++
		} else {
			if speculative >= 2 || (replacement.MarketValue <= 0 && replacement.ProjectedPoints <= 0 && replacement.RookieYear == 0) {
				continue
			}
			speculative++
		}
		salary := math.Max(snapshot.League.MinimumBid, replacement.EstimatedWinningBid)
		netRelief := candidate.SalaryCapRelief - salary
		evidence := "Established free agent with sufficient league-scored history."
		if !isEstablished {
			evidence = "Speculative option with limited history; dynasty rank or projection supplies the supporting signal."
		}
		if strings.EqualFold(replacement.AvailabilityStatus, "locked") {
			evidence += " MFL currently marks the player locked."
		}
		projectedPPG := 0.0
		projectedChange := 0.0
		if replacement.ProjectedPoints > 0 {
			projectedPPG = replacement.ProjectedPoints / 17
			evidence += " A current FantasyPros projection supplies a forward-looking role signal."
			if droppedProjection := snapshot.Projections.ByPlayerID[dropped.ID]; droppedProjection > 0 {
				projectedChange = projectedPPG - droppedProjection/17
			}
		}
		fitsBid := true
		if strings.Contains(strings.ToUpper(snapshot.League.WaiverType), "BBID") {
			fitsBid = salary <= snapshot.League.BlindBidBalance
		}
		option := ReplacementOption{
			ReplacementCandidate:       replacement,
			GrossCapRelief:             candidate.SalaryCapRelief,
			EstimatedAcquisitionSalary: round(salary, 2),
			NetCapRelief:               round(netRelief, 2),
			CapSpaceAfterTransaction:   round(capSpace+netRelief, 2),
			BlindBidBalanceAfter:       round(math.Max(0, snapshot.League.BlindBidBalance-salary), 2),
			ActiveRosterOpenAfter:      afterOpen,
			ProductionChange:           round(replacement.HistoricalPointsPerGame-candidate.ProductionValue, 2),
			ProjectedPointsPerGame:     round(projectedPPG, 2),
			ProjectedProductionChange:  round(projectedChange, 2),
			FitsSalaryCap:              capSpace+netRelief >= 0,
			FitsActiveRoster:           afterOpen >= 0,
			FitsBlindBidBudget:         fitsBid,
			BidEligibleNow:             !strings.EqualFold(replacement.AvailabilityStatus, "locked"),
			Evidence:                   evidence,
		}
		result = append(result, option)
		if established >= 3 && speculative >= 2 {
			break
		}
	}
	return result
}

func countStatus(players []Player, status string) int {
	count := 0
	for _, player := range players {
		if strings.EqualFold(player.Status, status) {
			count++
		}
	}
	return count
}

func developmentFactor(age, careerSeasons int) float64 {
	if careerSeasons <= 2 {
		switch {
		case age <= 23:
			return 1.75
		case age <= 26:
			return 1.50
		}
	}
	if careerSeasons <= 3 && age <= 26 {
		return 1.25
	}
	return 1
}

func disposition(candidate DropCandidate, vorp float64) (string, string) {
	if candidate.CareerSeasons <= 2 && candidate.Age <= 26 {
		return "hold_develop", "Early-career player is protected because historical production does not capture development or market value reliably."
	}
	if marketProtected(candidate) {
		return "trade_first", "Protected from an outright drop by current dynasty market ranking; trade before considering release."
	}
	if vorp >= 1 {
		return "trade_first", "Produces at least 1.0 PPG above the current positional replacement level."
	}
	return "drop_candidate", "At or near the current positional replacement level and not covered by the early-career protection rule."
}

func marketProtected(candidate DropCandidate) bool {
	return candidate.MarketValue >= 1000 || (candidate.DynastyRank > 0 && candidate.DynastyRank <= 100)
}

type cutPackage struct {
	cuts      []DropCandidate
	relief    int
	retention float64
}

func recommendCutPackage(candidates []DropCandidate, target float64) ([]DropCandidate, float64) {
	targetCents := int(math.Ceil(math.Max(0, target) * 100))
	if targetCents == 0 {
		return nil, 0
	}
	states := make([]*cutPackage, targetCents+1)
	states[0] = &cutPackage{}
	for _, candidate := range candidates {
		reliefCents := int(math.Round(candidate.SalaryCapRelief * 100))
		if reliefCents <= 0 {
			continue
		}
		next := append([]*cutPackage(nil), states...)
		for saved, state := range states {
			if state == nil {
				continue
			}
			newSaved := min(targetCents, saved+reliefCents)
			proposal := &cutPackage{
				cuts:      append(append([]DropCandidate(nil), state.cuts...), candidate),
				relief:    state.relief + reliefCents,
				retention: state.retention + candidate.RetentionValue,
			}
			if betterCutPackage(proposal, next[newSaved], targetCents) {
				next[newSaved] = proposal
			}
		}
		states = next
	}
	best := states[targetCents]
	if best == nil {
		return nil, 0
	}
	return best.cuts, round(float64(best.relief)/100, 2)
}

func betterCutPackage(candidate, current *cutPackage, targetCents int) bool {
	if current == nil {
		return true
	}
	if candidate.retention != current.retention {
		return candidate.retention < current.retention
	}
	candidateExcess, currentExcess := candidate.relief-targetCents, current.relief-targetCents
	if candidateExcess != currentExcess {
		return candidateExcess < currentExcess
	}
	return len(candidate.cuts) < len(current.cuts)
}

func dispositionPriority(value string) int {
	switch value {
	case "drop_candidate":
		return 0
	case "cap_efficiency_only":
		return 1
	case "trade_first":
		return 2
	case "hold_develop":
		return 3
	default:
		return 4
	}
}

func capRelief(player Player) float64 {
	switch player.Status {
	case "ROSTER":
		return player.Salary
	case "INJURED_RESERVE":
		if player.CurrentCapHit > 0 {
			return player.CurrentCapHit
		}
		return player.Salary * 0.5
	default:
		return 0
	}
}

func ageOn(birthdate, date time.Time) int {
	age := date.Year() - birthdate.Year()
	if date.Month() < birthdate.Month() || (date.Month() == birthdate.Month() && date.Day() < birthdate.Day()) {
		age--
	}
	return age
}

func ageFactor(position string, age int) float64 {
	peak := 29
	switch position {
	case "QB":
		peak = 35
	case "RB":
		peak = 26
	case "WR":
		peak = 28
	case "TE":
		peak = 29
	}
	if age < peak {
		return 1 + math.Min(0.24, float64(peak-age)*0.03)
	}
	return 1 - math.Min(0.50, float64(age-peak)*0.08)
}

func complianceScenarios(snapshot Snapshot, active, taxi []Player, mustLeave []PlayerSummary) []ComplianceScenario {
	if len(mustLeave) == 0 {
		return nil
	}
	playerByID := make(map[string]Player, len(taxi))
	for _, player := range taxi {
		playerByID[player.ID] = player
	}
	stashedCount := min(len(mustLeave), len(snapshot.Draft.CurrentYearPicks))
	stashedPicks := make([]string, 0, stashedCount)
	for i := 0; i < stashedCount; i++ {
		stashedPicks = append(stashedPicks, snapshot.Draft.CurrentYearPicks[i].Pick)
	}

	scenarios := make([]ComplianceScenario, 0, 1<<len(mustLeave))
	for mask := 0; mask < 1<<len(mustLeave); mask++ {
		scenario := ComplianceScenario{
			Promote:          []PlayerSummary{},
			RemoveOrTrade:    []PlayerSummary{},
			FirstPicksToTaxi: append([]string(nil), stashedPicks...),
		}
		promotedSalary := 0.0
		for i, player := range mustLeave {
			if mask&(1<<i) != 0 {
				scenario.Promote = append(scenario.Promote, player)
				promotedSalary += playerByID[player.ID].Salary
			} else {
				scenario.RemoveOrTrade = append(scenario.RemoveOrTrade, player)
			}
		}
		scenario.ResultingActive = len(active) + len(scenario.Promote)
		scenario.ResultingTaxi = len(taxi) - len(mustLeave) + stashedCount
		scenario.ResultingCapHit = round(snapshot.Franchise.TotalCapHit+promotedSalary, 2)
		scenario.AdditionalCapRelief = round(math.Max(0, scenario.ResultingCapHit-snapshot.League.SalaryCap), 2)
		scenario.RosterLegal = scenario.ResultingActive <= snapshot.League.ActiveRosterLimit &&
			scenario.ResultingTaxi <= snapshot.League.TaxiSquadLimit && scenario.AdditionalCapRelief == 0
		scenario.Name = scenarioName(scenario)
		scenarios = append(scenarios, scenario)
	}

	sort.Slice(scenarios, func(i, j int) bool {
		if len(scenarios[i].RemoveOrTrade) != len(scenarios[j].RemoveOrTrade) {
			return len(scenarios[i].RemoveOrTrade) < len(scenarios[j].RemoveOrTrade)
		}
		if scenarios[i].AdditionalCapRelief != scenarios[j].AdditionalCapRelief {
			return scenarios[i].AdditionalCapRelief < scenarios[j].AdditionalCapRelief
		}
		return scenarios[i].Name < scenarios[j].Name
	})
	return scenarios
}

func scenarioName(scenario ComplianceScenario) string {
	return fmt.Sprintf("promote %d, remove/trade %d", len(scenario.Promote), len(scenario.RemoveOrTrade))
}

func historicalEfficiency(snapshot Snapshot, active []Player) HistoricalEfficiency {
	values, seasons := weightedHistorical(snapshot.HistoricalPoints)
	efficiency := HistoricalEfficiency{
		Seasons: seasons,
		Method:  "game-weighted PPG with 4/3/2/1 season recency weights over up to four seasons",
		Caution: "Historical cap efficiency must not be interpreted as dynasty value or a cut ranking.",
	}
	players := make([]EfficiencyPlayer, 0, len(active))
	for _, player := range active {
		value, ok := values[player.ID]
		if !ok || player.Salary <= 0 {
			continue
		}
		players = append(players, EfficiencyPlayer{
			Name:                    player.Name,
			Position:                player.Position,
			Salary:                  player.Salary,
			HistoricalPointsPerGame: value.PointsPerGame,
			PointsPerGamePerSalary:  round(value.PointsPerGame/player.Salary, 2),
		})
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].PointsPerGamePerSalary != players[j].PointsPerGamePerSalary {
			return players[i].PointsPerGamePerSalary < players[j].PointsPerGamePerSalary
		}
		return players[i].Name < players[j].Name
	})
	limit := min(5, len(players))
	efficiency.LeastEfficient = append(efficiency.LeastEfficient, players[:limit]...)
	for i := len(players) - 1; i >= 0 && len(efficiency.MostEfficient) < limit; i-- {
		efficiency.MostEfficient = append(efficiency.MostEfficient, players[i])
	}
	return efficiency
}

type historicalValue struct {
	PointsPerGame float64
	Games         int
	Seasons       []int
}

func weightedHistorical(history HistoricalPoints) (map[string]historicalValue, []int) {
	series := append([]HistoricalSeason(nil), history.Seasons...)
	if len(series) == 0 && history.Season != 0 && len(history.ByPlayerID) > 0 {
		series = []HistoricalSeason{{Season: history.Season, ByPlayerID: history.ByPlayerID}}
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Season > series[j].Season })
	if len(series) > 4 {
		series = series[:4]
	}
	weightedPoints := make(map[string]float64)
	weightedGames := make(map[string]float64)
	totalGames := make(map[string]int)
	used := make(map[string][]int)
	seasons := make([]int, 0, len(series))
	for index, season := range series {
		weight := float64(4 - index)
		seasons = append(seasons, season.Season)
		for id, points := range season.ByPlayerID {
			games, ok := season.GamesPlayedByPlayerID[id]
			if !ok || games <= 0 {
				continue
			}
			weightedPoints[id] += points * weight
			weightedGames[id] += float64(games) * weight
			totalGames[id] += games
			used[id] = append(used[id], season.Season)
		}
	}
	result := make(map[string]historicalValue, len(weightedPoints))
	for id, points := range weightedPoints {
		result[id] = historicalValue{
			PointsPerGame: round(points/weightedGames[id], 2),
			Games:         totalGames[id],
			Seasons:       used[id],
		}
	}
	return result, seasons
}

func historicalSeasonList(values map[string]historicalValue) []int {
	seen := make(map[int]bool)
	for _, value := range values {
		for _, season := range value.Seasons {
			seen[season] = true
		}
	}
	seasons := make([]int, 0, len(seen))
	for season := range seen {
		seasons = append(seasons, season)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(seasons)))
	return seasons
}

func formatSeasons(seasons []int) string {
	if len(seasons) == 0 {
		return "no-season"
	}
	if len(seasons) == 1 {
		return strconv.Itoa(seasons[0])
	}
	return fmt.Sprintf("%d-%d", seasons[len(seasons)-1], seasons[0])
}

func playersWithStatus(players []Player, status string) []Player {
	result := make([]Player, 0)
	for _, player := range players {
		if player.Status == status {
			result = append(result, player)
		}
	}
	return result
}

func slotSummary(used, limit int) SlotSummary {
	return SlotSummary{Used: used, Limit: limit, Open: max(0, limit-used)}
}

func summarizePlayer(player Player) PlayerSummary {
	return PlayerSummary{ID: player.ID, Name: player.Name, Position: player.Position, Salary: player.Salary, RookieYear: player.RookieYear}
}

func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}
