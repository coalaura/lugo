fx_version 'cerulean'
client_script 'client.lua'
server_script 'server.lua'
shared_script 'shared.lua'

local manifestRuntimeCompletion = --[[@runtime_manifest_completion]]
local manifestWait = --[[@runtime_manifest_wait]]Wait
TriggerServerEvent(--[[@runtime_manifest_signature]]'runtime:event')
