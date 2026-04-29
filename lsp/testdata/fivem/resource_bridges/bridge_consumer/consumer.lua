local ping = exports.--[[@bridge_resource_completion]]bridge_provider.ping

local bridgeProxyHover = ping--[[@bridge_proxy_hover]]
local bridgeProxyCompletion = ping.--[[@bridge_proxy_completion]]

return bridgeProxyHover, bridgeProxyCompletion, ping(--[[@bridge_proxy_signature]]1), exports.bridge_provider:--[[@bridge_ping_call]]ping(--[[@bridge_export_signature]]1)
