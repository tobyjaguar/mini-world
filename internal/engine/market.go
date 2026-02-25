// Market resolution — hourly settlement-level trade between agents.
// See design doc Section 5.
package engine

import (
	"math"
	"sort"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/economy"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// Order represents a single buy or sell order in the market.
type Order struct {
	Agent    *agents.Agent
	Good     agents.GoodType
	Quantity int
	Price    float64
	IsSell   bool
}

// resolveMarkets runs market resolution for all settlements.
func (s *Simulation) resolveMarkets(tick uint64) {
	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		if len(settAgents) == 0 {
			continue
		}
		resolveSettlementMarket(sett, settAgents, tick, s.CurrentSeason)
	}
}

// resolveSettlementMarket aggregates supply/demand, resolves prices, and executes trades.
func resolveSettlementMarket(sett *social.Settlement, settAgents []*agents.Agent, tick uint64, season uint8) {
	market := sett.Market
	if market == nil {
		return
	}

	// Reset supply and demand.
	for _, entry := range market.Entries {
		entry.Supply = 0
		entry.Demand = 0
	}

	// Aggregate supply (surplus agents are willing to sell) and demand.
	for _, a := range settAgents {
		if !a.Alive {
			continue
		}

		// Supply: goods above the agent's personal threshold.
		for good, qty := range a.Inventory {
			surplus := qty - surplusThreshold(a, good)
			if surplus > 0 {
				if entry, ok := market.Entries[good]; ok {
					entry.Supply += float64(surplus)
				}
			}
		}

		// Demand: goods the agent needs but doesn't have enough of.
		for _, good := range demandedGoods(a) {
			if entry, ok := market.Entries[good]; ok {
				entry.Demand += 1
			}
		}
	}

	// Population-scaled supply floor prevents extreme demand/supply ratios
	// in large settlements. A 500-person settlement gets a floor of 5.
	supplyFloor := float64(sett.Population / 100)
	if supplyFloor < 1 {
		supplyFloor = 1
	}

	// Compute reference prices from supply/demand for order placement.
	// These drive ask/bid spreads but don't directly set entry.Price —
	// actual trades update prices via the clearing blend below.
	refPrices := make(map[agents.GoodType]float64)
	for good, entry := range market.Entries {
		if entry.Supply < supplyFloor {
			entry.Supply = supplyFloor
		}
		if entry.Demand < 1 {
			entry.Demand = 1
		}
		seasonMod := SeasonalMarketMod(season, uint8(good))
		refPrices[good] = entry.ResolvePrice(seasonMod, 1.0)
	}

	// Collect sell orders: agents with surplus above threshold.
	var sellOrders []Order
	for _, a := range settAgents {
		if !a.Alive {
			continue
		}
		for good, qty := range a.Inventory {
			surplus := qty - surplusThreshold(a, good)
			if surplus <= 0 {
				continue
			}
			ref, ok := refPrices[good]
			if !ok {
				continue
			}
			// Minimum acceptable price: reference price * Matter (~0.618).
			minPrice := ref * phi.Matter
			sellOrders = append(sellOrders, Order{
				Agent:    a,
				Good:     good,
				Quantity: surplus,
				Price:    minPrice,
				IsSell:   true,
			})
		}
	}

	// Collect buy orders: agents wanting goods they need.
	var buyOrders []Order
	for _, a := range settAgents {
		if !a.Alive {
			continue
		}
		for _, good := range demandedGoods(a) {
			ref, ok := refPrices[good]
			if !ok {
				continue
			}
			// Maximum willing price: reference price * Being (~1.618).
			maxPrice := ref * phi.Being
			buyOrders = append(buyOrders, Order{
				Agent:    a,
				Good:     good,
				Quantity: 1,
				Price:    maxPrice,
			})
		}
	}

	// Reset trade stats for this resolution cycle.
	market.TradeCount = 0
	maxTraded := 0

	// Match orders per good: sells ascending, buys descending, match until prices cross.
	for good, entry := range market.Entries {
		// Filter orders for this good.
		var sells []Order
		for _, o := range sellOrders {
			if o.Good == good {
				sells = append(sells, o)
			}
		}
		var buys []Order
		for _, o := range buyOrders {
			if o.Good == good {
				buys = append(buys, o)
			}
		}
		if len(sells) == 0 || len(buys) == 0 {
			continue
		}

		// Sort: sells ascending by price, buys descending by price.
		sort.Slice(sells, func(i, j int) bool { return sells[i].Price < sells[j].Price })
		sort.Slice(buys, func(i, j int) bool { return buys[i].Price > buys[j].Price })

		totalTraded := 0
		totalRevenue := 0.0
		si, bi := 0, 0
		sellRemain := 0
		if len(sells) > 0 {
			sellRemain = sells[0].Quantity
		}

		for si < len(sells) && bi < len(buys) {
			if sells[si].Price > buys[bi].Price {
				break // Prices crossed — no more matches.
			}
			// Clearing price: seller's ask price. Buyers accept the seller's
			// minimum — this prevents the midpoint formula from biasing prices
			// upward by (Matter+Being)/2 = 1.118x every tick.
			clearPrice := sells[si].Price
			clearCrowns := uint64(clearPrice + 0.5)
			// No minimum — 0-crown trades model barter in low-price economies.

			buyer := buys[bi].Agent
			seller := sells[si].Agent

			// Transfer one unit at a time.
			buyQty := buys[bi].Quantity
			tradeQty := buyQty
			if tradeQty > sellRemain {
				tradeQty = sellRemain
			}

			for u := 0; u < tradeQty; u++ {
				if clearCrowns > 0 && buyer.Wealth < clearCrowns {
					break
				}
				buyer.Wealth -= clearCrowns
				seller.Wealth += clearCrowns
				buyer.Inventory[good]++
				seller.Inventory[good]--
				totalTraded++
				totalRevenue += float64(clearCrowns)
				sellRemain--
			}

			// Advance buy pointer (buy orders are always qty 1).
			bi++

			// Advance sell pointer if exhausted.
			if sellRemain <= 0 {
				si++
				if si < len(sells) {
					sellRemain = sells[si].Quantity
				}
			}
		}

		// Update market price from clearing data, clamped to bounds.
		if totalTraded > 0 {
			avgClear := totalRevenue / float64(totalTraded)
			// Blend: 70% old price, 30% clearing price for stability.
			blended := entry.Price*0.7 + avgClear*0.3
			// Clamp to Phi-derived bounds — the blend must not break through.
			floor := entry.BasePrice * phi.Agnosis
			ceiling := entry.BasePrice * phi.Totality
			if blended < floor {
				blended = floor
			}
			if blended > ceiling {
				blended = ceiling
			}
			entry.Price = blended
		}

		// Accumulate trade stats for API consumption.
		market.TradeCount += totalTraded
		if totalTraded > maxTraded {
			maxTraded = totalTraded
			market.MostTradedGood = good
		}
	}
}

// surplusThreshold returns how many of a good an agent wants to keep before selling.
func surplusThreshold(a *agents.Agent, good agents.GoodType) int {
	switch good {
	case agents.GoodGrain, agents.GoodFish:
		// Keep some food — producers keep more, but not too much.
		if a.Occupation == agents.OccupationFarmer || a.Occupation == agents.OccupationFisher {
			return 3
		}
		return 2
	case agents.GoodIronOre, agents.GoodTimber:
		// Crafters keep raw materials.
		if a.Occupation == agents.OccupationCrafter {
			return 5
		}
		return 1
	case agents.GoodHerbs:
		if a.Occupation == agents.OccupationAlchemist {
			return 4
		}
		return 1
	case agents.GoodTools, agents.GoodWeapons:
		return 1 // Keep one for personal use
	default:
		return 1
	}
}

// demandedGoods returns which goods an agent wants to buy.
func demandedGoods(a *agents.Agent) []agents.GoodType {
	var needs []agents.GoodType

	// Everyone needs food — demand whichever is cheaper (grain or fish).
	food := a.Inventory[agents.GoodGrain] + a.Inventory[agents.GoodFish]
	if food < 3 {
		needs = append(needs, agents.GoodGrain)
		needs = append(needs, agents.GoodFish)
	}

	// Crafters demand materials for their best recipe only (max 2 goods).
	// Recipes: Tools (iron+timber), Weapons (iron+coal), Clothing (furs+tools), Luxuries (gems+tools).
	if a.Occupation == agents.OccupationCrafter {
		needs = append(needs, crafterRecipeDemand(a)...)
	}

	// Alchemists need herbs and exotics.
	if a.Occupation == agents.OccupationAlchemist {
		if a.Inventory[agents.GoodHerbs] < 2 {
			needs = append(needs, agents.GoodHerbs)
		}
		if a.Inventory[agents.GoodExotics] < 2 {
			needs = append(needs, agents.GoodExotics)
		}
	}

	// Everyone wants tools (improves work).
	if a.Inventory[agents.GoodTools] < 1 {
		needs = append(needs, agents.GoodTools)
	}

	return needs
}

// crafterRecipeDemand picks the recipe a crafter is closest to completing
// and returns demand for its missing materials. This prevents crafters from
// demanding all 5 raw materials simultaneously (which inflated raw material prices).
func crafterRecipeDemand(a *agents.Agent) []agents.GoodType {
	type recipe struct {
		mat1     agents.GoodType
		need1    int
		mat2     agents.GoodType
		need2    int
	}
	recipes := []recipe{
		{agents.GoodIronOre, 2, agents.GoodTimber, 1},  // Tools
		{agents.GoodIronOre, 2, agents.GoodCoal, 1},    // Weapons
		{agents.GoodFurs, 2, agents.GoodTools, 1},       // Clothing
		{agents.GoodGems, 2, agents.GoodTools, 1},       // Luxuries
	}

	// Score each recipe by how much inventory the crafter already has toward it.
	bestScore := -1
	bestIdx := 0
	for i, r := range recipes {
		have1 := a.Inventory[r.mat1]
		if have1 > r.need1 {
			have1 = r.need1
		}
		have2 := a.Inventory[r.mat2]
		if have2 > r.need2 {
			have2 = r.need2
		}
		score := have1 + have2
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	var needs []agents.GoodType
	r := recipes[bestIdx]
	if a.Inventory[r.mat1] < r.need1 {
		needs = append(needs, r.mat1)
	}
	if a.Inventory[r.mat2] < r.need2 {
		needs = append(needs, r.mat2)
	}
	return needs
}


// decayInventories runs goods decay for all agents in a settlement.
func (s *Simulation) decayInventories() {
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}
		agents.DecayInventory(a)
	}
}

// collectTaxes collects daily taxes from agents and applies settlement upkeep.
func (s *Simulation) collectTaxes(tick uint64) {
	taxThreshold := uint64(20) // Agents below this pay no tax

	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		taxRevenue := uint64(0)

		for _, a := range settAgents {
			if !a.Alive || a.Wealth <= taxThreshold {
				continue
			}
			taxable := a.Wealth - taxThreshold
			tax := uint64(float64(taxable) * sett.TaxRate * phi.Agnosis) // ~2.4% of taxable
			if tax < 1 {
				tax = 1
			}
			if tax > a.Wealth {
				tax = a.Wealth
			}
			a.Wealth -= tax
			taxRevenue += tax
		}

		sett.Treasury += taxRevenue

		// Settlement upkeep: population services and infrastructure maintenance.
		// Only population-based upkeep remains — treasury upkeep removed because
		// it destroyed crowns in the closed economy. Wealth decay now redirects
		// to treasury, and settlement wages push crowns back to agents.
		popUpkeep := uint64(float64(sett.Population) * phi.Agnosis * 0.5)
		if popUpkeep > sett.Treasury {
			popUpkeep = sett.Treasury
		}
		sett.Treasury -= popUpkeep
	}
}

// decayWealth applies a small daily wealth decay to all living agents.
// Models wear, loss, spoilage, and the friction of holding wealth.
// Decayed crowns flow into the agent's home settlement treasury —
// no crowns are destroyed. This keeps the money supply stable in the
// closed economy while still discouraging hoarding.
func (s *Simulation) decayWealth() {
	for _, a := range s.Agents {
		if !a.Alive || a.Wealth <= 20 {
			continue
		}
		// Progressive decay: rate scales logarithmically with wealth.
		// rate = Agnosis * 0.01 * (1 + Agnosis * log2(wealth/20))
		// At 20 crowns:    0.24%/day (unchanged baseline)
		// At 1,000:        0.56%/day
		// At 18,800 (avg): 0.80%/day
		// At 100,000:      0.94%/day
		// Compresses the top without destroying the economy.
		baseRate := phi.Agnosis * 0.01
		scaledRate := baseRate * (1.0 + phi.Agnosis*math.Log2(float64(a.Wealth)/20.0))
		decay := uint64(float64(a.Wealth-20) * scaledRate)
		if decay < 1 {
			decay = 1
		}
		a.Wealth -= decay
		// Redirect decayed crowns to home settlement treasury.
		if a.HomeSettID != nil {
			if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
				sett.Treasury += decay
			}
		}
	}
}

// paySettlementWages distributes a small daily wage from settlement treasuries
// to poor agents. This is a safety net, not primary income — agents should
// earn through market trade. Closes the treasury→agent loop.
func (s *Simulation) paySettlementWages() {
	// Compute global treasury/agent wealth ratio to dynamically target Φ⁻¹.
	// Target: treasury holds ~38.2% (1 - Matter), agents hold ~61.8% (Matter).
	totalTreasury := uint64(0)
	for _, sett := range s.Settlements {
		totalTreasury += sett.Treasury
	}
	totalAgent := s.Stats.TotalWealth // from previous updateStats()
	totalWealth := totalTreasury + totalAgent
	if totalWealth == 0 {
		return
	}

	treasuryShare := float64(totalTreasury) / float64(totalWealth)
	targetShare := 1.0 - phi.Matter // ~0.382 — treasury target

	// Scale outflow rate based on how far treasury is above target.
	// At target (38%): outflow = 1% baseline (maintenance flow).
	// At 50%: outflow ~4%. At 70%: outflow ~10%. At 80%+: outflow ~13%.
	// Below target: outflow = 0.5% (minimal, let taxes refill naturally).
	excess := treasuryShare - targetShare
	outflowRate := 0.005 // baseline when at or below target
	if excess > 0 {
		// Scale quadratically — gentle near target, aggressive when far above.
		outflowRate = 0.01 + excess*excess*40.0
		if outflowRate > phi.Agnosis { // cap at ~23.6%
			outflowRate = phi.Agnosis
		}
	}

	for _, sett := range s.Settlements {
		if sett.Treasury == 0 {
			continue
		}

		// Budget = outflowRate * treasury.
		budget := uint64(float64(sett.Treasury) * outflowRate)
		if budget < 2 {
			budget = 2
		}

		// Dynamic welfare threshold: Agnosis fraction (~24%) of settlement avg wealth.
		// At avg 18,800: threshold ≈ 4,437 — welfare reaches many more agents.
		// Minimum 50 to ensure welfare still flows in poor settlements.
		settAgents := s.SettlementAgents[sett.ID]
		avgWealth := 0.0
		aliveCount := 0
		for _, a := range settAgents {
			if a.Alive {
				avgWealth += float64(a.Wealth)
				aliveCount++
			}
		}
		if aliveCount > 0 {
			avgWealth /= float64(aliveCount)
		}
		threshold := avgWealth * phi.Agnosis
		if threshold < 50 {
			threshold = 50
		}
		totalWeight := 0.0
		for _, a := range settAgents {
			if a.Alive && a.Wealth < uint64(threshold) {
				totalWeight += (threshold - float64(a.Wealth)) / threshold
			}
		}
		if totalWeight == 0 {
			continue
		}

		paid := uint64(0)
		for _, a := range settAgents {
			if !a.Alive || a.Wealth >= uint64(threshold) {
				continue
			}
			// Progressive wage: proportional to how poor the agent is.
			weight := (threshold - float64(a.Wealth)) / threshold
			wage := uint64(float64(budget) * weight / totalWeight)
			if wage < 1 {
				wage = 1
			}
			if paid+wage > budget {
				break
			}
			if sett.Treasury < wage {
				break
			}
			sett.Treasury -= wage
			a.Wealth += wage
			paid += wage
		}
	}
}

// resolveBuyFood handles an agent's decision to buy food from the settlement market.
// Direct purchase: agent pays market price, gets 1 food, crowns go to treasury.
// This engages the economy — agents with wealth buy food instead of foraging.
func (s *Simulation) resolveBuyFood(a *agents.Agent) {
	if a.HomeSettID == nil {
		// No settlement — fall back to foraging behavior.
		a.Inventory[agents.GoodGrain]++
		a.Needs.Survival += 0.05
		return
	}
	sett, ok := s.SettlementIndex[*a.HomeSettID]
	if !ok || sett.Market == nil {
		a.Inventory[agents.GoodGrain]++
		a.Needs.Survival += 0.05
		return
	}

	// Find cheapest food available at the settlement market.
	var bestGood agents.GoodType
	bestPrice := float64(0)
	for _, good := range []agents.GoodType{agents.GoodGrain, agents.GoodFish} {
		entry, ok := sett.Market.Entries[good]
		if !ok {
			continue
		}
		price := entry.Price
		if bestPrice == 0 || price < bestPrice {
			bestPrice = price
			bestGood = good
		}
	}

	cost := uint64(bestPrice + 0.5)
	if cost < 1 {
		cost = 1
	}

	if a.Wealth >= cost {
		// Buy 1 unit of food — closed transfer to treasury.
		a.Wealth -= cost
		sett.Treasury += cost
		a.Inventory[bestGood]++
		// Buying food gives a small survival bump (anticipation of eating).
		a.Needs.Survival += 0.02
		a.Needs.Safety += 0.003    // "I can afford to eat" — economic security signal
		a.Needs.Belonging += 0.001 // Market participation is social.
		a.Needs.Purpose += 0.001   // Market participation is purposeful activity
	} else {
		// Can't afford — forage instead.
		a.Inventory[agents.GoodGrain]++
		a.Needs.Survival += 0.05
		a.Needs.Belonging += 0.001
	}
}

// resolveMerchantTrade lets merchants buy goods at home and sell at neighboring settlements.
// Called hourly after local market resolution.
func (s *Simulation) resolveMerchantTrade(tick uint64) {
	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		if sett.Market == nil {
			continue
		}

		// Find neighboring settlements within trade range (hex distance <= 5).
		var neighbors []*social.Settlement
		for _, other := range s.Settlements {
			if other.ID == sett.ID {
				continue
			}
			dist := world.Distance(sett.Position, other.Position)
			if dist <= 5 && other.Market != nil {
				neighbors = append(neighbors, other)
			}
		}
		if len(neighbors) == 0 {
			continue
		}

		for _, a := range settAgents {
			if !a.Alive || a.Occupation != agents.OccupationMerchant {
				continue
			}

			// Merchant still traveling — decrement travel ticks.
			if a.TravelTicksLeft > 0 {
				// Decrement by one hour's worth of ticks.
				if a.TravelTicksLeft <= TicksPerSimHour {
					a.TravelTicksLeft = 0
				} else {
					a.TravelTicksLeft -= uint16(TicksPerSimHour)
				}
				continue
			}

			// If merchant has cargo and arrived at destination, sell it.
			if a.TradeDestSett != nil && len(a.TradeCargo) > 0 {
				destSett, ok := s.SettlementIndex[*a.TradeDestSett]
				if ok && destSett.Market != nil {
					sellMerchantCargo(a, destSett.Market, destSett, s)
					s.Stats.TradeVolume++
				}
				// Repay consignment debt to home settlement treasury.
				if a.ConsignmentDebt > 0 {
					repay := a.ConsignmentDebt
					if repay > a.Wealth {
						repay = a.Wealth // Pay what you can.
					}
					a.Wealth -= repay
					sett.Treasury += repay
					a.ConsignmentDebt -= repay
				}
				a.TradeDestSett = nil
				a.TradeCargo = nil
				continue
			}

			// Look for profitable trade opportunity.
			bestProfit := 0.0
			var bestGood agents.GoodType
			var bestDest *social.Settlement

			for _, neighbor := range neighbors {
				for good, homeEntry := range sett.Market.Entries {
					destEntry, ok := neighbor.Market.Entries[good]
					if !ok {
						continue
					}
					if homeEntry.Price < 1 {
						continue
					}
					margin := (destEntry.Price - homeEntry.Price) / homeEntry.Price
					// Apply Being (Φ) as cooperation bonus.
					effectiveMargin := margin * phi.Being
					if effectiveMargin > phi.Psyche && effectiveMargin > bestProfit {
						bestProfit = effectiveMargin
						bestGood = good
						bestDest = neighbor
					}
				}
			}

			if bestDest == nil {
				continue
			}

			// Buy goods at home market.
			homeEntry := sett.Market.Entries[bestGood]
			buyPrice := uint64(homeEntry.Price + 0.5)
			if buyPrice < 1 {
				buyPrice = 1
			}
			// Buy up to 5 units: merchant pays from personal wealth first,
			// then home settlement treasury fronts the rest (consignment).
			// Consignment cost is tracked as debt repaid on sale.
			buyQty := 0
			for i := 0; i < 5; i++ {
				if a.Wealth >= buyPrice {
					a.Wealth -= buyPrice
					buyQty++
				} else if sett.Treasury >= buyPrice {
					sett.Treasury -= buyPrice
					a.ConsignmentDebt += buyPrice
					buyQty++
				} else {
					break
				}
			}
			if buyQty == 0 {
				continue
			}

			// Load cargo and set destination with travel time.
			destID := bestDest.ID
			a.TradeDestSett = &destID
			if a.TradeCargo == nil {
				a.TradeCargo = make(map[agents.GoodType]int)
			}
			a.TradeCargo[bestGood] += buyQty

			// Travel time based on terrain-aware route cost.
			travelCost := routeCost(sett.Position, bestDest.Position, s.WorldMap)
			if travelCost < 6 {
				travelCost = 6 // Minimum 1 hex worth of travel
			}
			a.TravelTicksLeft = uint16(travelCost)
		}
	}
}

// terrainMoveCost returns the tick cost to traverse one hex of the given terrain.
func terrainMoveCost(t world.Terrain) int {
	switch t {
	case world.TerrainPlains:
		return 6
	case world.TerrainForest:
		return 8
	case world.TerrainMountain:
		return 12
	case world.TerrainCoast:
		return 6
	case world.TerrainRiver:
		return 3
	case world.TerrainDesert:
		return 8
	case world.TerrainSwamp:
		return 10
	case world.TerrainTundra:
		return 8
	case world.TerrainOcean:
		return 999 // Impassable
	default:
		return 6
	}
}

// routeCost calculates the total tick cost to travel from one hex to another.
// Uses straight-line hex stepping (not full A*) for performance.
func routeCost(from, to world.HexCoord, worldMap *world.Map) int {
	cost := 0
	cur := from

	for cur != to {
		// Step toward destination: pick the neighbor closest to target.
		best := cur
		bestDist := world.Distance(cur, to)

		for _, n := range cur.Neighbors() {
			d := world.Distance(n, to)
			if d < bestDist {
				bestDist = d
				best = n
			}
		}

		if best == cur {
			// Shouldn't happen, but prevent infinite loop.
			break
		}

		cur = best
		hex := worldMap.Get(cur)
		if hex != nil {
			cost += terrainMoveCost(hex.Terrain)
		} else {
			cost += 6 // Default if hex not found
		}
	}

	return cost
}

// sellMerchantCargo sells a merchant's cargo at the destination settlement.
// Closed transfer: the settlement treasury pays the merchant per unit.
// After sale, Tier 2 merchants at the destination earn a commission (guild fee).
func sellMerchantCargo(a *agents.Agent, market *economy.Market, sett *social.Settlement, sim *Simulation) {
	var totalRevenue uint64
	for good, qty := range a.TradeCargo {
		entry, ok := market.Entries[good]
		if !ok || qty <= 0 {
			continue
		}
		for i := 0; i < qty; i++ {
			unitPrice := uint64(entry.Price + 0.5)
			if unitPrice < 1 {
				unitPrice = 1
			}
			if sett.Treasury < unitPrice {
				break // Treasury can't afford more.
			}
			sett.Treasury -= unitPrice
			a.Wealth += unitPrice
			totalRevenue += unitPrice
		}
	}

	// Successful trade gives merchant a sense of accomplishment.
	// Matches producer successful-work boosts from ResolveWork.
	if totalRevenue > 0 {
		a.Needs.Safety += 0.008
		a.Needs.Esteem += 0.012
		a.Needs.Belonging += 0.004
		a.Needs.Purpose += 0.004
	}

	// Tier 2 merchants at the destination earn a commission on trades
	// flowing through their settlement — guild masters who facilitate trade.
	if totalRevenue > 0 {
		tier2Commission(a, sett, totalRevenue, sim)
	}
}

// tier2Commission distributes a fraction of trade revenue to Tier 2 merchants
// in the destination settlement. Reflects their status as trade network facilitators.
// Closed transfer: selling merchant pays a guild fee, no crowns minted.
func tier2Commission(seller *agents.Agent, sett *social.Settlement, revenue uint64, sim *Simulation) {
	settAgents := sim.SettlementAgents[sett.ID]
	if len(settAgents) == 0 {
		return
	}

	// Cap total commission at Agnosis (~23.6%) of revenue so selling merchant still profits.
	maxTotalCommission := uint64(float64(revenue)*phi.Agnosis + 0.5)
	if maxTotalCommission == 0 {
		return
	}

	var totalPaid uint64
	for _, a := range settAgents {
		if a.Tier != agents.Tier2 || a.Occupation != agents.OccupationMerchant {
			continue
		}
		if !a.Alive || a.ID == seller.ID {
			continue // Don't commission yourself.
		}

		// Commission = revenue * Agnosis * 0.1 * (1 + coherence)
		coherence := float64(a.Soul.CittaCoherence)
		commission := uint64(float64(revenue) * phi.Agnosis * 0.1 * (1.0 + coherence))
		if commission == 0 {
			continue
		}

		// Respect per-trade cap.
		if totalPaid+commission > maxTotalCommission {
			commission = maxTotalCommission - totalPaid
		}
		// Seller must have enough wealth.
		if commission > seller.Wealth {
			commission = seller.Wealth
		}
		if commission == 0 {
			break
		}

		seller.Wealth -= commission
		a.Wealth += commission
		totalPaid += commission

		if totalPaid >= maxTotalCommission {
			break
		}
	}
}

// tier2MarketSell lets a Tier 2 agent sell their most valuable surplus good
// to the settlement treasury. Closed transfer — no crowns minted.
func tier2MarketSell(a *agents.Agent, sett *social.Settlement) {
	if sett.Market == nil {
		return
	}

	// Find the agent's most valuable surplus good.
	bestValue := 0.0
	bestGood := agents.GoodType(0)
	bestQty := 0
	for good, qty := range a.Inventory {
		surplus := qty - surplusThreshold(a, good)
		if surplus <= 0 {
			continue
		}
		entry, ok := sett.Market.Entries[good]
		if !ok {
			continue
		}
		value := float64(surplus) * entry.Price
		if value > bestValue {
			bestValue = value
			bestGood = good
			bestQty = surplus
		}
	}

	if bestQty == 0 {
		return
	}

	entry := sett.Market.Entries[bestGood]

	// Skill bonus: 1.0 + trade_skill * Agnosis, capped at Being.
	skillBonus := 1.0 + float64(a.Skills.Trade)*phi.Agnosis
	if skillBonus > phi.Being {
		skillBonus = phi.Being
	}

	for i := 0; i < bestQty; i++ {
		unitPrice := uint64(entry.Price*skillBonus + 0.5)
		if unitPrice < 1 {
			unitPrice = 1
		}
		if sett.Treasury < unitPrice {
			break
		}
		sett.Treasury -= unitPrice
		a.Wealth += unitPrice
		a.Inventory[bestGood]--
	}

	a.Skills.Trade += 0.005
}
