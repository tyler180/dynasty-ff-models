package analysis

import (
	"fmt"
	"strings"
)

func FormatText(analysis Analysis) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s — %s\n", analysis.Team, analysis.League)
	fmt.Fprintf(&out, "Snapshot: %s\n\n", analysis.SnapshotDate)
	fmt.Fprintf(&out, "CAP AND ROSTER\n")
	fmt.Fprintf(&out, "  Cap: $%.2f / $%.2f ($%.2f available)\n", analysis.Cap.Used, analysis.Cap.Limit, analysis.Cap.Space)
	fmt.Fprintf(&out, "  Active: %d / %d (%d open)\n", analysis.Roster.Active.Used, analysis.Roster.Active.Limit, analysis.Roster.Active.Open)
	fmt.Fprintf(&out, "  IR: %d / %d (%d open)\n", analysis.Roster.InjuredReserve.Used, analysis.Roster.InjuredReserve.Limit, analysis.Roster.InjuredReserve.Open)
	fmt.Fprintf(&out, "  Taxi: %d / %d (%d open)\n\n", analysis.Roster.Taxi.Used, analysis.Roster.Taxi.Limit, analysis.Roster.Taxi.Open)

	fmt.Fprintf(&out, "TAXI COMPLIANCE\n")
	if len(analysis.TaxiCompliance.MustLeaveTaxi) == 0 {
		fmt.Fprintf(&out, "  No known age-based taxi violations.\n")
	} else {
		fmt.Fprintf(&out, "  %d player(s) must leave taxi before compliance: %s.\n",
			len(analysis.TaxiCompliance.MustLeaveTaxi), playerList(analysis.TaxiCompliance.MustLeaveTaxi))
		fmt.Fprintf(&out, "  Moving them opens %d taxi slot(s), enough for the first %d pick(s).\n",
			analysis.TaxiCompliance.SlotsOpenedAfterMove,
			min(analysis.TaxiCompliance.SlotsOpenedAfterMove, len(analysis.Draft.Picks)))
	}
	for _, scenario := range analysis.ComplianceScenarios {
		status := "legal"
		if !scenario.RosterLegal {
			status = fmt.Sprintf("needs $%.2f additional cap relief", scenario.AdditionalCapRelief)
		}
		fmt.Fprintf(&out, "  - %s: promote [%s]; remove/trade [%s]; stash picks [%s] on taxi -> active %d, taxi %d, cap $%.2f (%s)\n",
			scenario.Name, playerList(scenario.Promote), playerList(scenario.RemoveOrTrade), strings.Join(scenario.FirstPicksToTaxi, ", "),
			scenario.ResultingActive, scenario.ResultingTaxi, scenario.ResultingCapHit, status)
	}
	fmt.Fprintln(&out)

	fmt.Fprintf(&out, "2026 DRAFT\n")
	fmt.Fprintf(&out, "  Status: %s", strings.ReplaceAll(analysis.Draft.Status, "_", " "))
	if analysis.Draft.AvailabilityWindow.Start != "" {
		fmt.Fprintf(&out, " (availability poll %s through %s)", analysis.Draft.AvailabilityWindow.Start, analysis.Draft.AvailabilityWindow.End)
	}
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "  %d picks; $%.2f total salary if every pick were used on an active player.\n", analysis.Draft.PickCount, analysis.Draft.TotalSalaryIfActive)
	for _, pick := range analysis.Draft.Picks {
		active := "fits active now"
		if !pick.FitsActiveNow {
			if pick.ActiveCapShortfall > 0 {
				active = fmt.Sprintf("active needs $%.2f cap relief", pick.ActiveCapShortfall)
			} else {
				active = "active needs a roster slot"
			}
		}
		taxi := "fits taxi now"
		if !pick.FitsTaxiNow {
			taxi = "taxi needs a slot"
		}
		fmt.Fprintf(&out, "  - %s (#%d), $%.2f: %s; %s\n", pick.Pick, pick.Overall, pick.Salary, active, taxi)
	}
	fmt.Fprintln(&out)

	fmt.Fprintf(&out, "ROOKIE BOARDS\n")
	fmt.Fprintf(&out, "  Coverage: %d ranked, %d unranked\n", analysis.RookieBoard.RankedCandidates, analysis.RookieBoard.UnrankedCandidates)
	writeRookiePool(&out, "OFFENSE", analysis.RookieBoard.Offense)
	writeRookiePool(&out, "IDP", analysis.RookieBoard.IDP)
	if analysis.RookieBoard.Other != nil {
		writeRookiePool(&out, "OTHER POSITIONS", *analysis.RookieBoard.Other)
	}
	fmt.Fprintf(&out, "  Caution: %s\n\n", analysis.RookieBoard.Caution)

	fmt.Fprintf(&out, "%s WEIGHTED CAP EFFICIENCY (CONTEXT ONLY)\n", formatSeasons(analysis.HistoricalEfficiency.Seasons))
	fmt.Fprintf(&out, "  Method: %s\n", analysis.HistoricalEfficiency.Method)
	fmt.Fprintf(&out, "  Most efficient: %s\n", efficiencyList(analysis.HistoricalEfficiency.MostEfficient))
	fmt.Fprintf(&out, "  Least efficient: %s\n", efficiencyList(analysis.HistoricalEfficiency.LeastEfficient))
	fmt.Fprintf(&out, "  Caution: %s\n\n", analysis.HistoricalEfficiency.Caution)

	fmt.Fprintf(&out, "CAP-CUT EVALUATION\n")
	if !analysis.DropEvaluation.Available {
		fmt.Fprintf(&out, "  Unavailable: pass -projections with player-ID values, or opt into -projection-fallback historical.\n\n")
	} else {
		fmt.Fprintf(&out, "  Production input: %s\n", analysis.DropEvaluation.ProjectionSource)
		if analysis.DropEvaluation.ReplacementSource != "" {
			fmt.Fprintf(&out, "  Replacement input: %s\n", analysis.DropEvaluation.ReplacementSource)
		}
		if len(analysis.DropEvaluation.RecommendedCuts) > 0 {
			names := make([]string, 0, len(analysis.DropEvaluation.RecommendedCuts))
			for _, candidate := range analysis.DropEvaluation.RecommendedCuts {
				names = append(names, candidate.Name)
			}
			fmt.Fprintf(&out, "  Recommended DROP package for at least $%.2f relief: %s — saves $%.2f\n",
				analysis.DropEvaluation.CapReliefTarget, strings.Join(names, ", "), analysis.DropEvaluation.RecommendedRelief)
		} else if analysis.DropEvaluation.CapReliefTarget <= 0 {
			fmt.Fprintf(&out, "  No cap relief is currently required.\n")
		} else {
			fmt.Fprintf(&out, "  No combination of players classified as DROP candidates provides the $%.2f target relief; review trade-first options instead.\n", analysis.DropEvaluation.CapReliefTarget)
		}
		writeCandidateSection(&out, "DROP CANDIDATES", analysis.DropEvaluation.DropCandidates, analysis.DropEvaluation.ProductionMetric)
		writeCandidateSection(&out, "TRADE FIRST — DO NOT DROP", analysis.DropEvaluation.TradeFirst, analysis.DropEvaluation.ProductionMetric)
		writeCandidateSection(&out, "HOLD / DEVELOP", analysis.DropEvaluation.HoldDevelop, analysis.DropEvaluation.ProductionMetric)
		fmt.Fprintf(&out, "  Caution: %s\n\n", analysis.DropEvaluation.Caution)
	}

	fmt.Fprintf(&out, "LIMITATIONS\n")
	for _, warning := range analysis.Warnings {
		fmt.Fprintf(&out, "  - %s\n", warning)
	}
	return out.String()
}

func writeRookiePool(out *strings.Builder, label string, pool RookieBoardPool) {
	fmt.Fprintf(out, "  %s (%d ranked, %d unranked):\n", label, pool.RankedCandidates, pool.UnrankedCandidates)
	limit := min(10, pool.RankedCandidates)
	for _, candidate := range pool.Candidates[:limit] {
		fmt.Fprintf(out, "    - #%d %s (%s", candidate.Rank, candidate.Name, candidate.Position)
		if candidate.NFLTeam != "" {
			fmt.Fprintf(out, ", %s", candidate.NFLTeam)
		}
		details := []string{}
		if candidate.RookieRank > 0 {
			details = append(details, fmt.Sprintf("rookie ECR %.1f", candidate.RookieRank))
		}
		if candidate.RookieADP > 0 {
			details = append(details, fmt.Sprintf("rookie ADP %.2f", candidate.RookieADP))
		}
		if candidate.ConsensusRankScore > 0 {
			details = append(details, fmt.Sprintf("consensus score %.2f", candidate.ConsensusRankScore))
		}
		if candidate.DynastyRank > 0 {
			details = append(details, fmt.Sprintf("dynasty ECR %.1f", candidate.DynastyRank))
		}
		if candidate.MarketValue > 0 {
			details = append(details, fmt.Sprintf("market value %.0f", candidate.MarketValue))
		}
		if candidate.ProjectedPoints > 0 {
			details = append(details, fmt.Sprintf("projected points %.1f", candidate.ProjectedPoints))
		}
		fmt.Fprintf(out, "): %s\n", strings.Join(details, "; "))
	}
}

func writeCandidateSection(out *strings.Builder, label string, candidates []DropCandidate, metric string) {
	if len(candidates) == 0 {
		return
	}
	limit := min(10, len(candidates))
	fmt.Fprintf(out, "  %s (%d):\n", label, len(candidates))
	for _, candidate := range candidates[:limit] {
		if candidate.Disposition == "cap_efficiency_only" {
			fmt.Fprintf(out, "    - %s (%s, age %d): $%.2f relief / %.2f %s / %.3f score\n",
				candidate.Name, candidate.Position, candidate.Age, candidate.SalaryCapRelief, candidate.ProductionValue, metric, candidate.DropScore)
			continue
		}
		switch candidate.Disposition {
		case "drop_candidate":
			fmt.Fprintf(out, "    - %s (%s, age %d): $%.2f relief; %.2f PPG vs %.2f replacement (VORP %+.2f); %.3f drop score\n",
				candidate.Name, candidate.Position, candidate.Age, candidate.SalaryCapRelief, candidate.ProductionValue,
				candidate.ReplacementPointsPerGame, candidate.ValueOverReplacement, candidate.DropScore)
		case "trade_first":
			fmt.Fprintf(out, "    - %s (%s, age %d): $%.2f relief; %.2f PPG vs %.2f replacement (VORP %+.2f); trade before cutting\n",
				candidate.Name, candidate.Position, candidate.Age, candidate.SalaryCapRelief, candidate.ProductionValue,
				candidate.ReplacementPointsPerGame, candidate.ValueOverReplacement)
		case "hold_develop":
			fmt.Fprintf(out, "    - %s (%s, age %d): %d season(s), %.2f development factor; hold unless an explicit dynasty valuation says otherwise\n",
				candidate.Name, candidate.Position, candidate.Age, candidate.CareerSeasons, candidate.DevelopmentFactor)
		}
	}
}

func playerList[T interface{ ~[]PlayerSummary }](players T) string {
	if len(players) == 0 {
		return "none"
	}
	labels := make([]string, 0, len(players))
	for _, player := range players {
		labels = append(labels, fmt.Sprintf("%s ($%.0f)", player.Name, player.Salary))
	}
	return strings.Join(labels, ", ")
}

func efficiencyList(players []EfficiencyPlayer) string {
	if len(players) == 0 {
		return "unavailable"
	}
	labels := make([]string, 0, len(players))
	for _, player := range players {
		labels = append(labels, fmt.Sprintf("%s %.2f PPG/$", player.Name, player.PointsPerGamePerSalary))
	}
	return strings.Join(labels, "; ")
}
