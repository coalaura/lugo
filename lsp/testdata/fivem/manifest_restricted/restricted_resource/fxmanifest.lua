fx_version 'cerulean'
game 'gta5'

client_script 'client.lua'
server_script 'server.lua'
shared_script 'shared.lua'

local restrictedManifestCompletion = --[[@restricted_manifest_completion]]
local restrictedManifestWait = --[[@restricted_manifest_wait]]Wait
local restrictedManifestSource = --[[@restricted_manifest_source]]source
local restrictedManifestExports = --[[@restricted_manifest_exports]]exports
TriggerServerEvent(--[[@restricted_manifest_signature]]'restricted:event')
