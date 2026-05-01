package lsp

import (
	"fmt"
	"testing"
)

func attachTestFiveMNativeBundleLoader(tb testing.TB, s *Server) {
	tb.Helper()
	if s == nil {
		tb.Fatal("server is nil")
	}
	s.fiveMNativeBundleLoader = newTestFiveMNativeBundleLoader(tb)
}

func newTestFiveMNativeBundleLoader(tb testing.TB) func(name string) ([]byte, error) {
	tb.Helper()

	bundles := map[string]string{
		"natives_universal.lua": `---@meta

---**PLAYER client**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD80958FC74E988A6)  
---Returns the entity handle for the local player ped.
---@return integer
function PlayerPedId() end

---**VEHICLE client**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA2D4EAB7A8B5E5A0)  
---Returns the maximum number of passengers supported by the vehicle model.
---@param vehicle integer
---@return integer
function GetVehicleMaxNumberOfPassengers(vehicle) end

---**CFX shared**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4D52FE5B)  
---Returns the currently invoking resource name when available.
---@return string
function GetInvokingResource() end
`,
		"natives_0193d0af.lua": `---@meta

---**VEHICLE client**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA2D4EAB7A8B5E5A0)  
---Returns the maximum number of passengers supported by the vehicle model.
---@param vehicle integer
---@return integer
function GetVehicleMaxNumberOfPassengers(vehicle) end
`,
		"natives_21e43a33.lua": `---@meta

---**VEHICLE client**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB215AAC32D25D019)  
---Returns the display name for a vehicle model.
---@param model integer
---@return string
function GetDisplayNameFromVehicleModel(model) end
`,
		"natives_server.lua": `---@meta

---**CFX server**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4D52FE5B)  
---Returns the currently invoking resource name when available.
---@return string
function GetInvokingResource() end
`,
		"rdr3_universal.lua": `---@meta

---**PLAYER client**  
---[Native Documentation](https://rdr3natives.com/?_0x275F255ED201B937)  
---Returns a player ped handle for the given player index.
---@param playerIndex integer
---@return integer
function GetPlayerPed(playerIndex) end
`,
		"ny_universal.lua": `---@meta

---Returns the world position for the specified character.
---@param charHandle integer
---@return any
function GetCharCoordinates(charHandle) end
`,
	}

	return func(name string) ([]byte, error) {
		content, ok := bundles[name]
		if !ok {
			return nil, fmt.Errorf("missing test FiveM native bundle %s", name)
		}
		return []byte(content), nil
	}
}
