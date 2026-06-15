package components

import (
  "fmt"
  "github.com/freiheit-com/kuberpult/assets/css"
)

type Screen struct {
  // ... existing fields ...
  BackgroundClass string
}

func NewScreen() *Screen {
  return &Screen{
    BackgroundClass: "screen-background",
    // ... existing initialization ...
  }
}
