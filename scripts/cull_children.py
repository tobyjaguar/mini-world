#!/usr/bin/env python3
"""
C2 Offline Population Pruning Script

One-time script to reduce world population from 494K to 400K by randomly
culling children (age < 16) with uniform probability across all ages.

Operates directly on the SQLite database. Does NOT run the simulation.
Agents are marked alive=0 (same as natural death) — not deleted.

Usage:
    python3 scripts/cull_children.py [--db path/to/crossworlds.db] [--target 400000] [--dry-run] [--seed 42]
"""

import argparse
import random
import sqlite3
import sys
from collections import Counter


def main():
    parser = argparse.ArgumentParser(description="Cull children to reduce population")
    parser.add_argument("--db", default="data/crossworlds.db", help="Path to SQLite database")
    parser.add_argument("--target", type=int, default=400_000, help="Target population")
    parser.add_argument("--dry-run", action="store_true", help="Show what would happen without modifying DB")
    parser.add_argument("--seed", type=int, default=42, help="Random seed for reproducibility")
    args = parser.parse_args()

    conn = sqlite3.connect(args.db)
    conn.execute("PRAGMA journal_mode=WAL")
    c = conn.cursor()

    # --- Snapshot current state ---
    c.execute("SELECT COUNT(*) FROM agents WHERE alive = 1")
    total_alive = c.fetchone()[0]

    c.execute("SELECT age, COUNT(*) FROM agents WHERE alive = 1 GROUP BY age ORDER BY age")
    age_dist = {row[0]: row[1] for row in c.fetchall()}

    children_total = sum(cnt for age, cnt in age_dist.items() if age < 16)
    adults_total = sum(cnt for age, cnt in age_dist.items() if age >= 16)
    to_cull = total_alive - args.target

    print(f"=== Population Pruning Script ===")
    print(f"Database: {args.db}")
    print(f"Current population: {total_alive:,}")
    print(f"Target population:  {args.target:,}")
    print(f"Children (<16):     {children_total:,} ({100*children_total/total_alive:.1f}%)")
    print(f"Adults (>=16):      {adults_total:,} ({100*adults_total/total_alive:.1f}%)")
    print(f"To cull:            {to_cull:,}")
    print(f"Random seed:        {args.seed}")
    print()

    if to_cull <= 0:
        print("Population already at or below target. Nothing to do.")
        return

    if to_cull > children_total:
        print(f"ERROR: Need to cull {to_cull:,} but only {children_total:,} children exist.")
        print("Would need to cull adults too. Aborting.")
        return

    cull_rate = to_cull / children_total
    print(f"Cull rate: {to_cull:,} / {children_total:,} = {100*cull_rate:.1f}% of children")
    print()

    # --- Select children to cull (uniform random across all ages) ---
    c.execute("SELECT id, age FROM agents WHERE alive = 1 AND age < 16")
    all_children = c.fetchall()

    rng = random.Random(args.seed)
    rng.shuffle(all_children)
    victims = all_children[:to_cull]

    # --- Report age distribution of culled agents ---
    cull_by_age = Counter(age for _, age in victims)
    print("Age distribution of culled agents:")
    print(f"  {'Age':>4}  {'Culled':>8}  {'Total':>8}  {'% Culled':>8}")
    print(f"  {'---':>4}  {'------':>8}  {'-----':>8}  {'--------':>8}")
    for age in sorted(age_dist.keys()):
        if age >= 16:
            continue
        total_at_age = age_dist[age]
        culled_at_age = cull_by_age.get(age, 0)
        pct = 100 * culled_at_age / total_at_age if total_at_age > 0 else 0
        print(f"  {age:4d}  {culled_at_age:8,}  {total_at_age:8,}  {pct:7.1f}%")

    total_culled = sum(cull_by_age.values())
    print(f"  {'':>4}  {'------':>8}")
    print(f"  {'Tot':>4}  {total_culled:8,}")
    print()

    # --- Verify post-cull population ---
    remaining = total_alive - total_culled
    print(f"Post-cull population: {remaining:,}")
    print(f"Adults preserved:     {adults_total:,} (100%)")
    print()

    if args.dry_run:
        print("DRY RUN — no changes made.")
        return

    # --- Confirm ---
    print(f"This will mark {total_culled:,} agents as dead (alive=0) in {args.db}")
    response = input("Proceed? [y/N] ")
    if response.lower() != 'y':
        print("Aborted.")
        return

    # --- Execute cull ---
    victim_ids = [id for id, _ in victims]

    # SQLite has a limit on variables per query, batch in chunks of 999
    batch_size = 999
    culled = 0
    for i in range(0, len(victim_ids), batch_size):
        batch = victim_ids[i:i + batch_size]
        placeholders = ",".join("?" * len(batch))
        c.execute(f"UPDATE agents SET alive = 0 WHERE id IN ({placeholders})", batch)
        culled += c.rowcount

    conn.commit()
    print(f"\nDone. Marked {culled:,} agents as dead.")

    # --- Verify ---
    c.execute("SELECT COUNT(*) FROM agents WHERE alive = 1")
    final_pop = c.fetchone()[0]
    print(f"Final population: {final_pop:,}")

    conn.close()


if __name__ == "__main__":
    main()
