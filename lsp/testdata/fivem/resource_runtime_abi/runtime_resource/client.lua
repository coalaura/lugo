local runtimeClientCompletion = --[[@runtime_client_completion]]
Citizen.--[[@runtime_citizen_wait_hover]]Wait(0)
RegisterNUICallback--[[@runtime_nui_callback_hover]]('runtime:ping', function(data, reply)
	local runtimeReplyHover = reply--[[@runtime_nui_reply_hover]]
	local runtimeReplyCompletion = reply.--[[@runtime_nui_reply_completion]]
	runtimeReplyHover(--[[@runtime_nui_reply_signature]]{ ok = true, data = data })
	return runtimeReplyCompletion
end)
local exportBridge = --[[@runtime_exports_hover]]exports
TriggerServerEvent('runtime:event', --[[@runtime_trigger_server_signature]]42)
local state = GlobalState
local packed = msgpack.pack({ ok = true, state = state })

return exportBridge, packed
