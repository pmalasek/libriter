package plugin

import "time"

// SetTimeout zkrátí strop volání (testy).
func (h *Host) SetTimeout(d time.Duration) { h.timeout = d }
