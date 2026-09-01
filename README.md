# Dynasty Fantasy Football Models

Provider-independent Go models for dynasty roster and rookie-draft decisions.
They evaluate roster cuts, salary-cap constraints, taxi eligibility,
replacement value, starting-lineup value, future contracts, reserved picks,
and dated eligibility changes.

Data acquisition and persistence live in the sibling `dynasty-ff-backend`
repository. This module contains no MFL, AWS, database, or deployment code.

Public packages:

- `draft` provides deterministic rookie-draft optimization through
  `draft.Recommend`.
- `analysis` provides roster, cap, taxi, historical-efficiency,
  replacement- and dynasty-market-aware drop analysis, including multi-player
  cap-relief packages and transaction-level current-free-agent replacement
  options, and independently ranked offense and IDP rookie boards
  through `analysis.AnalyzeWithOptions`. Each board uses a
  60/40 blend of rookie ECR and board-relative rookie-only ADP when both are
  available, and either signal can rank deeper candidates on its own.
- `usage` detects sustained increases or decreases in defensive snap share from
  canonical game-level facts through `usage.Analyze`.

Callers build the public inputs from their own data stores and provider facts.

## Use it as a Go library

```go
import draftmodel "github.com/tyler180/dynasty-ff-models/draft"

result, err := draftmodel.Recommend(input)
```

```go
import analysismodel "github.com/tyler180/dynasty-ff-models/analysis"

result := analysismodel.AnalyzeWithOptions(snapshot, options)
```

```go
import usagemodel "github.com/tyler180/dynasty-ff-models/usage"

report, err := usagemodel.Analyze(input)
```

The backend resolves canonical player IDs, loads league state and historical
features, and builds `draftmodel.Input`. The model validates and evaluates that
input without knowing where it came from.

## Defensive usage trends

The usage model compares the three most recent games with the preceding three
games by default. Each window is weighted from raw totals:

```text
defensive snap share = sum(player defensive snaps) / sum(team defensive snaps)
```

A change of at least 10 percentage points must appear in at least two recent
games before the model emits `rising` or `falling`. A large window change caused
by only one game is `volatile`; fewer than six valid regular-season games is
`insufficient_data`. Strong changes of at least 20 points that appear in every
recent game receive high confidence.

The output retains weekly observations, baseline and recent window totals,
percentage-point change, confirmation count, confidence, and a plain-language
reason. Thresholds and window sizes are configurable. The model does not fetch
PFR/nflverse data or resolve provider IDs; the backend supplies canonical facts.
A complete JSON input is available at `data/usage-example.json`.

## How decisions are scored

For each available rookie, the optimizer tries every allowed combination of
zero through `league.max_drops` droppable roster players. An option is rejected
if it violates any season's salary cap, roster limit, or positional limit.

Each feasible roster receives this season score:

```text
starter projected points
+ bench_weight * non-starter projected points
- salary_penalty * total salary
```

Future scores are multiplied by
`discount_rate^(year-current_year)`. Starting lineups are optimized separately
for each season, allowing future eligibility changes to affect today's choice.

## Input model

See `data/example.json` for a complete example.

- `remaining_draft_picks[0]` is the current pick and supplies the selected
  rookie's salary unless the player has a year-specific override.
- Later picks reserve salary and, by default, a roster spot.
- `projected_points` and `salary` are keyed by season.
- `contract_through` removes a player after that season.
- `droppable` defaults to true; set it false for protected players.
- `max_by_position` is optional, and multi-eligible players are assigned where
  needed to make the roster legal.
- Eligibility rules add positions and do not remove existing eligibility.

## Test

```sh
go test ./...
```

The current optimizer recommends the selection at the first remaining pick.
Later picks are reserved for cap and roster planning; opponent simulations and
full draft-tree optimization remain future extensions.
