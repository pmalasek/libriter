// Package handler obsahuje HTTP handlery jádra: přihlášení, tokeny a správu
// uživatelů. Jsou společné všem produktům.
package handler

import "mediater/internal/core/httpx"

var (
	writeJSON  = httpx.WriteJSON
	writeError = httpx.WriteError
	readJSON   = httpx.ReadJSON
	parseUUID  = httpx.ParseUUID
)
