AddEventHandler('runtime:event', function() end)
AddStateBagChangeHandler(nil, nil, function(--[[@runtime_statebag_bag_hover]]bagName, key, value, source, --[[@runtime_statebag_replicated_hover]]replicated)
	return bagName, key, value, source, replicated
end)

local runtimeSharedCompletion = --[[@runtime_shared_completion]]
TriggerServerEvent(--[[@runtime_shared_signature]]'runtime:event')
