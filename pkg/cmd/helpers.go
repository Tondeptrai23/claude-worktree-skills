package cmd

import (
	"fmt"
	"strconv"
)

func parseSlot(s string, maxSlots int) (int, error) {
	slot, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid slot number: %s", s)
	}
	if slot < 1 || slot > maxSlots {
		return 0, fmt.Errorf("slot must be 1-%d, got %d", maxSlots, slot)
	}
	return slot, nil
}
