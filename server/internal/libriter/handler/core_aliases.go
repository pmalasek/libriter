package handler

import (
	corehandler "mediater/internal/core/handler"
	"mediater/internal/core/httpx"
)

// Pomocníci jádra pod původními jmény, ať se knižní handlery nemusí měnit.
var (
	writeJSON           = httpx.WriteJSON
	writeError          = httpx.WriteError
	readJSON            = httpx.ReadJSON
	readImageBody       = httpx.ReadImageBody
	parseUUID           = httpx.ParseUUID
	writeUserInputError = corehandler.WriteUserInputError
	userLabel           = corehandler.UserLabel
)

const minPasswordLength = corehandler.MinPasswordLength
