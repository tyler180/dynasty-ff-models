// Package draft exposes the stable, provider-independent API for dynasty
// rookie draft optimization. Callers assemble Input from their own data stores;
// this package has no knowledge of MFL, DynamoDB, S3, or Lambda.
package draft

import (
	"github.com/tylermclean/dynasty-ff-models/internal/model"
	"github.com/tylermclean/dynasty-ff-models/internal/optimizer"
)

type Input = model.Input
type League = model.League
type Season = model.Season
type LineupSlot = model.LineupSlot
type Player = model.Player
type DraftPick = model.DraftPick
type EligibilityRule = model.EligibilityRule
type Recommendation = model.Recommendation
type CandidateRanking = model.CandidateRanking
type SeasonSummary = model.SeasonSummary

// Recommend evaluates every available rookie and allowed drop combination over
// the configured season horizon and returns the highest-scoring feasible plan.
func Recommend(input Input) (Recommendation, error) {
	return optimizer.Recommend(input)
}
