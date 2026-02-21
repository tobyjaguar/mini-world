# Project Vision: Mini-World

## What We're Building

A persistent, autonomous simulated world that runs on a cloud server. Agents (villagers, creatures, etc.) go about their lives — forming relationships, making decisions, responding to weather and events — without any human intervention. Users can check in at any time to see what's happened, read the event log, and observe the current state of the world.

## Design Pillars

1. **Autonomy**: The world runs itself. No player input needed to advance.
2. **Emergence**: Complex, interesting behavior arises from simple rules and agent interactions.
3. **Persistence**: State is saved continuously. Restart the server and the world picks up where it left off.
4. **Observability**: Rich event logging makes it easy to catch up on what happened while you were away.
5. **Groundedness**: Real weather data and true randomness make the world feel alive and unpredictable.

## Key Questions (For Research)

- What world size and agent count produces interesting dynamics without overwhelming compute?
- How frequently should LLM calls be made vs. rule-based decisions? (Cost vs. richness tradeoff)
- What agent memory architecture works best? (Full history vs. summarized vs. sliding window)
- What's the right tick rate? (Real-time vs. accelerated)
- How do we prevent agent behavior from becoming repetitive or degenerate over time?

## Inspiration

- Stanford "Generative Agents" (2023) — LLM-powered agents in a simulated town
- Dwarf Fortress — emergent storytelling from complex rule systems
- The Sims — needs-based agent behavior
- Conway's Game of Life — complexity from simplicity
- Project Sid — large-scale agent civilization

## Status

Project initiated. Awaiting research results from Claude.ai on simulated world design patterns and prior art before finalizing architecture and language choice.
