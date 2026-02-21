// Market resolution — hourly settlement-level trade between agents.
// See design doc Section 5.
package engine

import (
	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/economy"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

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

	// Resolve prices from supply/demand.
	for good, entry := range market.Entries {
		// Ensure minimum supply/demand so prices don't go to extremes immediately.
		if entry.Supply < 1 {
			entry.Supply = 1
		}
		if entry.Demand < 1 {
			entry.Demand = 1
		}
		seasonMod := SeasonalMarketMod(season, uint8(good))
		entry.Price = entry.ResolvePrice(seasonMod, 1.0)
	}

	// Execute trades: sellers sell surplus, buyers buy needs.
	for _, a := range settAgents {
		if !a.Alive {
			continue
		}
		executeTrades(a, market)
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

	// Crafters need raw materials.
	if a.Occupation == agents.OccupationCrafter {
		if a.Inventory[agents.GoodIronOre] < 2 {
			needs = append(needs, agents.GoodIronOre)
		}
		if a.Inventory[agents.GoodTimber] < 1 {
			needs = append(needs, agents.GoodTimber)
		}
	}

	// Alchemists need herbs.
	if a.Occupation == agents.OccupationAlchemist {
		if a.Inventory[agents.GoodHerbs] < 2 {
			needs = append(needs, agents.GoodHerbs)
		}
	}

	// Everyone wants tools (improves work).
	if a.Inventory[agents.GoodTools] < 1 {
		needs = append(needs, agents.GoodTools)
	}

	return needs
}

// executeTrades has an agent sell surplus and buy needs at market prices.
func executeTrades(a *agents.Agent, market *economy.Market) {
	// Sell surplus.
	for good, qty := range a.Inventory {
		surplus := qty - surplusThreshold(a, good)
		if surplus <= 0 {
			continue
		}
		entry, ok := market.Entries[good]
		if !ok {
			continue
		}
		// Sell up to surplus amount.
		sellQty := surplus
		revenue := uint64(float64(sellQty) * entry.Price)
		a.Inventory[good] -= sellQty
		a.Wealth += revenue
	}

	// Buy needs.
	for _, good := range demandedGoods(a) {
		entry, ok := market.Entries[good]
		if !ok {
			continue
		}
		price := uint64(entry.Price + 0.5) // Round to nearest crown
		if price < 1 {
			price = 1
		}
		// Buy one unit at a time if affordable.
		if a.Wealth >= price {
			a.Wealth -= price
			a.Inventory[good]++
		}
	}
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

		// Settlement upkeep: population * Agnosis * 0.5 crowns/day.
		upkeep := uint64(float64(sett.Population) * phi.Agnosis * 0.5)
		if upkeep > sett.Treasury {
			upkeep = sett.Treasury
		}
		sett.Treasury -= upkeep
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

			// If merchant has cargo and a destination, sell it there.
			if a.TradeDestSett != nil && len(a.TradeCargo) > 0 {
				destSett, ok := s.SettlementIndex[*a.TradeDestSett]
				if ok && destSett.Market != nil {
					sellMerchantCargo(a, destSett.Market)
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

			// Load cargo and set destination.
			destID := bestDest.ID
			a.TradeDestSett = &destID
			if a.TradeCargo == nil {
				a.TradeCargo = make(map[agents.GoodType]int)
			}
			a.TradeCargo[bestGood] += buyQty
		}
	}
}

// sellMerchantCargo sells a merchant's cargo at the destination market.
func sellMerchantCargo(a *agents.Agent, market *economy.Market) {
	for good, qty := range a.TradeCargo {
		entry, ok := market.Entries[good]
		if !ok || qty <= 0 {
			continue
		}
		revenue := uint64(float64(qty) * entry.Price)
		a.Wealth += revenue
	}
}
