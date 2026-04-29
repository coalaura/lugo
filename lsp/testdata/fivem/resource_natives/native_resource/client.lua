local ped = --[[@native_client_call]]PlayerPedId()
local legacyOnly = --[[@native_client_legacy_hidden]]GetVehicleMaxNumberOfPassengers
local serverOnly = --[[@native_client_server_hidden]]GetInvokingResource

return ped, legacyOnly, serverOnly
