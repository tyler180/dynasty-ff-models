# Dynasty Rookie Draft Model

A deterministic, provider-independent Go optimizer for dynasty rookie draft
decisions. It evaluates available rookies, required roster cuts, salary-cap
constraints, starting-lineup value, bench value, positional limits, future
contracts, reserved picks, and dated eligibility changes across a configured
time horizon.

Data acquisition and persistence live in the sibling `dynasty-ff-backend`
repository. This module contains no MFL, AWS, database, or deployment code.

## Use it as a Go library

```go
import draftmodel "github.com/tylermclean/dynasty-ff-models/draft"

result, err := draftmodel.Recommend(input)
```

The backend resolves canonical player IDs, loads league state and historical
features, and builds `draftmodel.Input`. The model validates and evaluates that
input without knowing where it came from.

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
