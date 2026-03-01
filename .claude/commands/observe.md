# Observe Crossworlds — Deity-Level World Analysis

You are an observer of the autonomous world simulation "Crossworlds". Pull live state from both the API and the local SQLite database, analyze the world's health, and surface insights and improvement suggestions.

## Data Sources

### 1. API Endpoints (live running state)
Use `https://api.crossworlds.xyz` as the base URL (WebFetch auto-upgrades HTTP→HTTPS, which breaks raw IP access since nginx serves plain HTTP on :80). Fetch these endpoints using WebFetch:

- `https://api.crossworlds.xyz/api/v1/status` — tick, population, mood, wealth, season, per-occupation counts + satisfaction
- `https://api.crossworlds.xyz/api/v1/economy` — prices, inflation/deflation, wealth distribution, trade volume, producer health
- `https://api.crossworlds.xyz/api/v1/stats/history?limit=10` — recent trends (includes occupation_json snapshots)
- `https://api.crossworlds.xyz/api/v1/settlements` — all settlements with governance and health

For factions, use `curl` instead of WebFetch (the raw response is too large due to per-settlement influence maps):
```bash
curl -s 'https://api.crossworlds.xyz/api/v1/factions' | python3 -c "
import json, sys
for f in json.load(sys.stdin):
    print(f\"{f['name']:25s} treasury={f.get('treasury',0):>12.0f}  members={f.get('members',0):>6.0f}\")
"
```

### 2. SQLite Database Queries (deeper analysis)
The production database is remote, but a local copy may exist at `data/crossworlds.db`. If it exists, run analytical queries using `sqlite3 data/crossworlds.db`. Key queries:

```sql
-- Needs distribution: how many agents have each need above/below thresholds
SELECT
  SUM(CASE WHEN json_extract(needs_json, '$.Belonging') > 0.3 THEN 1 ELSE 0 END) as belonging_ok,
  SUM(CASE WHEN json_extract(needs_json, '$.Belonging') <= 0.3 THEN 1 ELSE 0 END) as belonging_low,
  SUM(CASE WHEN json_extract(needs_json, '$.Purpose') > 0.3 THEN 1 ELSE 0 END) as purpose_ok,
  SUM(CASE WHEN json_extract(needs_json, '$.Purpose') <= 0.3 THEN 1 ELSE 0 END) as purpose_low
FROM agents WHERE alive = 1;

-- Mood by occupation
SELECT occupation, COUNT(*) as n, AVG(mood) as avg_mood, MIN(mood) as min_mood
FROM agents WHERE alive = 1 GROUP BY occupation ORDER BY avg_mood;

-- Wealth distribution by occupation
SELECT occupation, COUNT(*) as n, AVG(wealth) as avg_wealth, MAX(wealth) as max_wealth
FROM agents WHERE alive = 1 GROUP BY occupation ORDER BY avg_wealth DESC;

-- Faction membership counts
SELECT faction_id, COUNT(*) as members
FROM agents WHERE alive = 1 AND faction_id IS NOT NULL
GROUP BY faction_id ORDER BY faction_id;

-- Faction treasury state
SELECT id, name, treasury FROM factions;

-- Death rate trend (from stats_history)
SELECT tick, population, deaths, avg_mood, avg_survival FROM stats_history ORDER BY tick DESC LIMIT 20;

-- Agents with extreme states (very rich, very poor mood, zero needs)
SELECT name, occupation, wealth, mood,
  json_extract(needs_json, '$.Safety') as safety,
  json_extract(needs_json, '$.Belonging') as belonging,
  json_extract(needs_json, '$.Purpose') as purpose
FROM agents WHERE alive = 1
ORDER BY mood ASC LIMIT 20;
```

## Analysis Framework

After gathering data, analyze across these dimensions:

### Economic Health
- Are prices converging toward base prices or stuck at ceiling/floor?
- Is wealth distribution getting more or less equal (Gini trend)?
- Are any goods permanently scarce or permanently oversupplied?
- Trade volume trend — is inter-settlement trade active?

### Agent Well-Being
- Average mood trend — improving, stable, or declining?
- Needs distribution — what percentage of agents have belonging, purpose, esteem above 0.3?
- Occupation-level mood — are any occupations systematically unhappy?
- Death rate — is it declining now that tuning fixes are in?

### Political Balance
- Faction treasury accumulation — are dues being collected and persisted?
- Faction influence spread — does each faction have strongholds?
- Crown presence in monarchies vs Merchant's Compact in merchant republics
- Are faction tensions generating events?

### Population Dynamics
- Birth/death ratio — is population stable, growing, or declining?
- Age distribution — are agents aging and dying of old age, or mostly starvation?
- Migration — are agents moving between settlements?

## Output Format

Present findings as a concise world report with:
1. **World State Summary** — one paragraph on the current state
2. **Key Metrics** — table of critical numbers with trend arrows if history available
3. **Health Assessment** — what's working well, what's concerning
4. **Suggested Improvements** — specific, actionable tuning or feature recommendations ranked by impact
5. **Things to Monitor** — what to check next session
