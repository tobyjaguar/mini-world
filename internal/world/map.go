package world

import "fmt"

// Map holds the complete hex grid world state.
type Map struct {
	Hexes  map[HexCoord]*Hex `json:"-"` // All hexes keyed by coordinate
	Radius int               `json:"radius"`
}

// NewMap creates an empty map with the given radius.
// A hex grid of radius R contains hexes where max(|q|, |r|, |s|) <= R.
func NewMap(radius int) *Map {
	m := &Map{
		Hexes:  make(map[HexCoord]*Hex),
		Radius: radius,
	}
	return m
}

// Get returns the hex at the given coordinate, or nil if out of bounds.
func (m *Map) Get(coord HexCoord) *Hex {
	return m.Hexes[coord]
}

// Set places a hex at the given coordinate.
func (m *Map) Set(hex *Hex) {
	m.Hexes[hex.Coord] = hex
}

// InBounds returns true if the coordinate is within the map radius.
func (m *Map) InBounds(coord HexCoord) bool {
	q := coord.Q
	r := coord.R
	s := coord.S()
	if q < 0 {
		q = -q
	}
	if r < 0 {
		r = -r
	}
	if s < 0 {
		s = -s
	}
	max := q
	if r > max {
		max = r
	}
	if s > max {
		max = s
	}
	return max <= m.Radius
}

// HexCount returns the total number of hexes in the map.
func (m *Map) HexCount() int {
	return len(m.Hexes)
}

// String returns a summary of the map.
func (m *Map) String() string {
	return fmt.Sprintf("Map(radius=%d, hexes=%d)", m.Radius, m.HexCount())
}
