package components

import (
  "fmt"
  "github.com/freiheit-com/kuberpult/assets/css"
)

type Tile struct {
  // ... existing fields ...
  BackgroundClass string
}

func NewTile() *Tile {
  return &Tile{
    BackgroundClass: "tile-background",
    // ... existing initialization ...
  }
}
