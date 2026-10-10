package plugin

import "time"

// SetTimeout zkrátí strop volání (testy).
func (h *Host) SetTimeout(d time.Duration) { h.timeout = d }

// SetBackoff zkrátí pauzu po odmítnutí zdrojem (testy).
func (h *Host) SetBackoff(lo, hi time.Duration) { h.backoffMin, h.backoffMax = lo, hi }
