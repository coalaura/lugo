local resourceName = --[[@native_server_call]]GetInvokingResource()
local clientOnly = --[[@native_server_client_hidden]]PlayerPedId

return resourceName, clientOnly
