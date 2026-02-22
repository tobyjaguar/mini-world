// Market resolution — hourly settlement-level trade between agents.
// See design doc Section 5.
package engine

import (
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

	// Resolve prices from supply/demand.
	for good, entry := range market.Entries {
		// Ensure minimum supply/demand so prices don't go to extremes immediately.
		if entry.Supply < supplyFloor {
			entry.Supply = supplyFloor
		}
		if entry.Demand < 1 {
			entry.Demand = 1
		}
		seasonMod := SeasonalMarketMod(season, uint8(good))
		entry.Price = entry.ResolvePrice(seasonMod, 1.0)
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
			entry, ok := market.Entries[good]
			if !ok {
				continue
			}
			// Minimum acceptable price: current price * Matter (~0.38).
			minPrice := entry.Price * phi.Matter
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
			entry, ok := market.Entries[good]
			if !ok {
				continue
			}
			// Maximum willing price: current price * Being (~1.618).
			maxPrice := entry.Price * phi.Being
			buyOrders = append(buyOrders, Order{
				Agent:    a,
				Good:     good,
				Quantity: 1,
				Price:    maxPrice,
			})
		}
	}

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
			// Clearing price: midpoint of ask and bid.
			clearPrice := (sells[si].Price + buys[bi].Price) / 2
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

		// Update market price from clearing data.
		if totalTraded > 0 {
			avgClear := totalRevenue / float64(totalTraded)
			// Blend: 70% old price, 30% clearing price for stability.
			entry.Price = entry.Price*0.7 + avgClear*0.3
		}
	}
}

// surplusThreshold returns how many of a good an agent wants to keep before selling.
func surplusThreshold(a *agents.Agent, good agents.GoodType) int {
	switch good {
	case agents.GoodGrain, agents.GoodFish:
		// Keep some food — farmers keep more.
		if a.Occupation == agents.OccupationFarmer || a.Occupation == agents.OccupationFisher {
			return 5
		}
		return 3
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

	// Everyone needs food.
	food := a.Inventory[agents.GoodGrain] + a.Inventory[agents.GoodFish]
	if food < 3 {
		needs = append(needs, agents.GoodGrain)
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

		// Settlement upkeep has two components:
		// 1. Population upkeep: services, infrastructure maintenance.
		// 2. Treasury upkeep: bureaucracy, waste, corruption — scales with wealth.
		//    Acts as a wealth sink in the closed economy, preventing
		//    infinite treasury accumulation.
		popUpkeep := uint64(float64(sett.Population) * phi.Agnosis * 0.5)
		treasuryUpkeep := uint64(float64(sett.Treasury) * phi.Agnosis * 0.01)
		upkeep := popUpkeep + treasuryUpkeep
		if upkeep > sett.Treasury {
			upkeep = sett.Treasury
		}
		sett.Treasury -= upkeep
	}
}

// decayWealth applies a small daily wealth decay to all living agents.
// Models wear, loss, spoilage, and the friction of holding wealth.
// Acts as a complementary sink in the closed economy, ensuring crowns
// circulate rather than accumulate.
func (s *Simulation) decayWealth() {
	for _, a := range s.Agents {
		if !a.Alive || a.Wealth <= 20 {
			continue
		}
		// Lose ~0.24% of wealth above 20 crowns per day (Agnosis * 0.01).
		// At 1000 crowns this is ~2/day; at 100k it's ~236/day.
		decay := uint64(float64(a.Wealth-20) * phi.Agnosis * 0.01)
		if decay < 1 {
			decay = 1
		}
		a.Wealth -= decay
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
					sellMerchantCargo(a, destSett.Market, destSett)
					s.Stats.TradeVolume++
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
			// Buy up to 5 units or what the merchant can afford.
			buyQty := 0
			for i := 0; i < 5; i++ {
				if a.Wealth >= buyPrice {
					a.Wealth -= buyPrice
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
func sellMerchantCargo(a *agents.Agent, market *economy.Market, sett *social.Settlement) {
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
