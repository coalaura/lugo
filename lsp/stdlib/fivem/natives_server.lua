---@meta

---Generated from live rage-lua-natives-compatible metadata for `natives_server.lua`.
---Source input: `natives_cfx.json` (server server).
---Generated from live CFX server/shared native metadata.
---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC6F43D0E)  
---Creates a blip for the specified coordinates. You can use `SET_BLIP_` natives to change the blip.
---
---**This is the server-side RPC native equivalent of the client native [ADD_BLIP_FOR_COORD](?\_0x5A039BB0BCA604B6).**
---@param x number
---@param y number
---@param z number
---@return integer
function AddBlipForCoord(x, y, z) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x30822554)  
---Create a blip that by default is red (enemy), you can use [SET_BLIP_AS_FRIENDLY](#\_0xC6F43D0E) to make it blue (friend).\
---Can be used for objects, vehicles and peds.
---Example of enemy:
---![enemy](https://i.imgur.com/fl78svv.png)
---Example of friend:
---![friend](https://i.imgur.com/Q16ho5d.png)
---
---**This is the server-side RPC native equivalent of the client native [ADD_BLIP_FOR_ENTITY](?\_0x5CDE92C702A8FCE7).**
---@param entity integer
---@return integer
function AddBlipForEntity(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4626756C)  
---Create a blip with a radius for the specified coordinates (it doesnt create the blip sprite, so you need to use [AddBlipCoords](#\_0xC6F43D0E))
---Example image:
---![example](https://i.imgur.com/9hQl3DB.png)
---
---**This is the server-side RPC native equivalent of the client native [ADD_BLIP_FOR_RADIUS](?\_0x46818D79B1F7499A).**
---@param posX number
---@param posY number
---@param posZ number
---@param radius number
---@return integer
function AddBlipForRadius(posX, posY, posZ, radius) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xAB7F7241)  
---Adds a listener for Console Variable changes.
---
---The function called expects to match the following signature:
---
---```ts
---function ConVarChangeListener(conVarName: string, reserved: any);
---```
---
---*   **conVarName**: The ConVar that changed.
---*   **reserved**: Currently unused.
---@param conVarFilter string
---@param handler function
---@return integer
function AddConvarChangeListener(conVarFilter, handler) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x70559AC7)  
---```
---Applies an Item from a PedDecorationCollection to a ped. These include tattoos and shirt decals.
---collection - PedDecorationCollection filename hash
---overlay - Item name hash
---Example:
---Entry inside "mpbeach_overlays.xml" -
---<Item>
---<uvPos x="0.500000" y="0.500000" />
---<scale x="0.600000" y="0.500000" />
---<rotation value="0.000000" />
---<nameHash>FM_Hair_Fuzz</nameHash>
---<txdHash>mp_hair_fuzz</txdHash>
---<txtHash>mp_hair_fuzz</txtHash>
---<zone>ZONE_HEAD</zone>
---<type>TYPE_TATTOO</type>
---<faction>FM</faction>
---<garment>All</garment>
---<gender>GENDER_DONTCARE</gender>
---<award />
---<awardLevel />
---</Item>
---Code:
---PED::_0x5F5D1665E352A839(PLAYER::PLAYER_PED_ID(), MISC::GET_HASH_KEY("mpbeach_overlays"), MISC::GET_HASH_KEY("fm_hair_fuzz"))
---```
---
---**This is the server-side RPC native equivalent of the client native [ADD_PED_DECORATION_FROM_HASHES](?\_0x5F5D1665E352A839).**
---@param ped integer
---@param collection integer | string
---@param overlay integer | string
function AddPedDecorationFromHashes(ped, collection, overlay) end

---@deprecated
_applyPedOverlay = AddPedDecorationFromHashes

---@deprecated
_setPedDecoration = AddPedDecorationFromHashes

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x5BA35AAF)  
---Adds a handler for changes to a state bag.
---
---The function called expects to match the following signature:
---
---```ts
---function StateBagChangeHandler(bagName: string, key: string, value: any, reserved: number, replicated: boolean);
---```
---
---*   **bagName**: The internal bag ID for the state bag which changed. This is usually `player:Source`, `entity:NetID`
---    or `localEntity:Handle`.
---*   **key**: The changed key.
---*   **value**: The new value stored at key. The old value is still stored in the state bag at the time this callback executes.
---*   **reserved**: Currently unused.
---*   **replicated**: Whether the set is meant to be replicated.
---
---At this time, the change handler can't opt to reject changes.
---
---If bagName refers to an entity, use [GET_ENTITY_FROM_STATE_BAG_NAME](#\_0x4BDF1867) to get the entity handle
---If bagName refers to a player, use [GET_PLAYER_FROM_STATE_BAG_NAME](#\_0xA56135E0) to get the player handle
---@param keyFilter string
---@param bagFilter string
---@param handler function
---@return integer
function AddStateBagChangeHandler(keyFilter, bagFilter, handler) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC1C0855A)  
---```cpp
---enum eApplyForceTypes {
---APPLY_TYPE_FORCE = 0,
---APPLY_TYPE_IMPULSE = 1,
---APPLY_TYPE_EXTERNAL_FORCE = 2,
---APPLY_TYPE_EXTERNAL_IMPULSE = 3,
---APPLY_TYPE_TORQUE = 4,
---APPLY_TYPE_ANGULAR_IMPULSE = 5
---}
---```
---
---**This is the server-side RPC native equivalent of the client native [APPLY_FORCE_TO_ENTITY](?\_0xC5F68BE9613E2D18).**
---@param entity integer
---@param forceType integer
---@param x number
---@param y number
---@param z number
---@param offX number
---@param offY number
---@param offZ number
---@param nComponent integer
---@param bLocalForce boolean
---@param bLocalOffset boolean
---@param bScaleByMass boolean
---@param bPlayAudio boolean
---@param bScaleByTimeWarp boolean
function ApplyForceToEntity(entity, forceType, x, y, z, offX, offY, offZ, nComponent, bLocalForce, bLocalOffset, bScaleByMass, bPlayAudio, bScaleByTimeWarp) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x429461C3)  
---Returns whether or not the specified player has enough information to start a commerce session for.
---@param playerSrc string
---@return boolean
function CanPlayerStartCommerceSession(playerSrc) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFA29D35D)  
---Cancels the currently executing event.
function CancelEvent() end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2D23D743)  
---CLEAR_PED_PROP
---
---**This is the server-side RPC native equivalent of the client native [CLEAR_PED_PROP](?\_0x0943E5B8E078E76E).**
---@param ped integer
---@param propId integer
function ClearPedProp(ped, propId) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA635F451)  
---CLEAR_PED_SECONDARY_TASK
---
---**This is the server-side RPC native equivalent of the client native [CLEAR_PED_SECONDARY_TASK](?\_0x176CECF6F920D707).**
---@param ped integer
function ClearPedSecondaryTask(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDE3316AB)  
---Clear a ped's tasks. Stop animations and other tasks created by scripts.
---
---**This is the server-side RPC native equivalent of the client native [CLEAR_PED_TASKS](?\_0xE1EF3C1216AFF2CD).**
---@param ped integer
function ClearPedTasks(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xBC045625)  
---Immediately stops the pedestrian from whatever it's doing. The difference between this and [CLEAR_PED_TASKS](#\_0xE1EF3C1216AFF2CD) is that this one teleports the ped but does not change the position of the ped.
---
---**This is the server-side RPC native equivalent of the client native [CLEAR_PED_TASKS_IMMEDIATELY](?\_0xAAA34F8A7CB32098).**
---@param ped integer
function ClearPedTasksImmediately(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x54EA5BCC)  
---```
---This executes at the same as speed as PLAYER::SET_PLAYER_WANTED_LEVEL(player, 0, false);
---PLAYER::GET_PLAYER_WANTED_LEVEL(player); executes in less than half the time. Which means that it's worth first checking if the wanted level needs to be cleared before clearing. However, this is mostly about good code practice and can important in other situations. The difference in time in this example is negligible.
---```
---
---**This is the server-side RPC native equivalent of the client native [CLEAR_PLAYER_WANTED_LEVEL](?\_0xB302540597885499).**
---@param player integer
function ClearPlayerWantedLevel(player) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2F7AA05C)  
---Creates an object (prop) with the specified model at the specified position, offset on the Z axis by the radius of the object's model.
---This object will initially be owned by the creating script as a mission entity, and the model should be loaded already (e.g. using REQUEST_MODEL).
---
---**This is the server-side RPC native equivalent of the client native [CREATE_OBJECT](?\_0x509D5878EB39E842).**
---@param modelHash integer | string
---@param x number
---@param y number
---@param z number
---@param isNetwork boolean
---@param netMissionEntity boolean
---@param doorFlag boolean
---@return integer
function CreateObject(modelHash, x, y, z, isNetwork, netMissionEntity, doorFlag) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x58040420)  
---Creates an object (prop) with the specified model centered at the specified position.
---This object will initially be owned by the creating script as a mission entity, and the model should be loaded already (e.g. using REQUEST_MODEL).
---
---**This is the server-side RPC native equivalent of the client native [CREATE_OBJECT_NO_OFFSET](?\_0x9A294B2138ABB884).**
---@param modelHash integer | string
---@param x number
---@param y number
---@param z number
---@param isNetwork boolean
---@param netMissionEntity boolean
---@param doorFlag boolean
---@return integer
function CreateObjectNoOffset(modelHash, x, y, z, isNetwork, netMissionEntity, doorFlag) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x389EF71)  
---Creates a ped (biped character, pedestrian, actor) with the specified model at the specified position and heading.
---This ped will initially be owned by the creating script as a mission entity, and the model should be loaded already
---(e.g. using REQUEST_MODEL).
---
---**This is the server-side RPC native equivalent of the client native [CREATE_PED](?\_0xD49F9B0955C367DE).**
---@param pedType integer
---@param modelHash integer | string
---@param x number
---@param y number
---@param z number
---@param heading number
---@param isNetwork boolean
---@param bScriptHostPed boolean
---@return integer
function CreatePed(pedType, modelHash, x, y, z, heading, isNetwork, bScriptHostPed) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x3000F092)  
---CREATE_PED_INSIDE_VEHICLE
---
---**This is the server-side RPC native equivalent of the client native [CREATE_PED_INSIDE_VEHICLE](?\_0x7DD959874C1FD534).**
---@param vehicle integer
---@param pedType integer
---@param modelHash integer | string
---@param seat integer
---@param isNetwork boolean
---@param bScriptHostPed boolean
---@return integer
function CreatePedInsideVehicle(vehicle, pedType, modelHash, seat, isNetwork, bScriptHostPed) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDD75460A)  
---Creates a vehicle with the specified model at the specified position. This vehicle will initially be owned by the creating
---script as a mission entity, and the model should be loaded already (e.g. using REQUEST_MODEL).
---
---```
---NativeDB Added Parameter 8: BOOL p7
---```
---
---**This is the server-side RPC native equivalent of the client native [CREATE_VEHICLE](?\_0xAF35D0D2583051B0).**
---@param modelHash integer | string
---@param x number
---@param y number
---@param z number
---@param heading number
---@param isNetwork boolean
---@param netMissionEntity boolean
---@return integer
function CreateVehicle(modelHash, x, y, z, heading, isNetwork, netMissionEntity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6AE51D4B)  
---Equivalent to CREATE_VEHICLE, but it uses 'server setter' logic (like the former CREATE_AUTOMOBILE) as a workaround for
---reliability concerns regarding entity creation RPC.
---
---Unlike CREATE_AUTOMOBILE, this supports other vehicle types as well.
---@param modelHash integer | string
---@param type string
---@param x number
---@param y number
---@param z number
---@param heading number
---@return integer
function CreateVehicleServerSetter(modelHash, type, x, y, z, heading) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFAA3D236)  
---Deletes the specified entity.
---
---**NOTE**: For trains this will only work if called on the train engine, it will not work on its carriages.
---@param entity integer
function DeleteEntity(entity) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1E86F206)  
---This native does not have an official description.
---@param referenceIdentity string
function DeleteFunctionReference(referenceIdentity) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7389B5DF)  
---This native does not have an official description.
---@param key string
function DeleteResourceKvp(key) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4152C90)  
---Nonsynchronous [DELETE_RESOURCE_KVP](#\_0x7389B5DF) operation; see [FLUSH_RESOURCE_KVP](#\_0x5240DA5A).
---@param key string
function DeleteResourceKvpNoSync(key) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x523BA3DA)  
---Deletes the specified `entity` and any carriage its attached to, or that is attached to it.
---@param entity integer
function DeleteTrain(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x43F15989)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function DoesBoatSinkWhenWrecked(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x3AC90869)  
---This native does not have an official description.
---@param entity integer
---@return boolean
function DoesEntityExist(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x12038599)  
---Returns whether or not the player exists
---@param playerSrc string
---@return boolean
function DoesPlayerExist(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x167ABA27)  
---Requests whether or not the player owns the specified SKU.
---@param playerSrc string
---@param skuId integer
---@return boolean
function DoesPlayerOwnSku(playerSrc, skuId) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDEF0480B)  
---Requests whether or not the player owns the specified package.
---@param playerSrc string
---@param skuId integer
---@return boolean
function DoesPlayerOwnSkuExt(playerSrc, skuId) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x77CC80DC)  
---This native does not have an official description.
---@param train integer
---@return boolean
function DoesTrainStopAtStations(train) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xBA0613E1)  
---This native does not have an official description.
---@param playerSrc string
---@param reason string
function DropPlayer(playerSrc, reason) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF4E2079D)  
---This native does not have an official description.
---@param referenceIdentity string
---@return string
function DuplicateFunctionReference(referenceIdentity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF97B1C93)  
---This native does not have an official description.
---@param enabled boolean
function EnableEnhancedHostSupport(enabled) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB3210203)  
---This native does not have an official description.
---@param handle integer
function EndFindKvp(handle) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x3BB78F05)  
---Internal function for ensuring an entity has a state bag.
---@param entity integer
function EnsureEntityStateBag(entity) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x561C060B)  
---Depending on your use case you may need to use `add_acl resource.<your_resource_name> command.<command_name> allow` to use this native in your resource.
---@param commandString string
function ExecuteCommand(commandString) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xBD7BEBC5)  
---This native does not have an official description.
---@param handle integer
---@return string
function FindKvp(handle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x13B6855D)  
---This native does not have an official description.
---@param private_ boolean
function FlagServerAsPrivate(private_) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE27C97A0)  
---Nonsynchronous operations will not wait for a disk/filesystem flush before returning from a write or delete call. They will be much faster than their synchronous counterparts (e.g., bulk operations), however, a system crash may lose the data to some recent operations.
---
---This native ensures all `_NO_SYNC` operations are synchronized with the disk/filesystem.
function FlushResourceKvp() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD70C3BCA)  
---An internal function for converting a stack trace object to a string.
---@param traceData table
---@return string
function FormatStackTrace(traceData) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x65C16D57)  
---Freezes or unfreezes an entity preventing its coordinates to change by the player if set to `true`. You can still change the entity position using [`SET_ENTITY_COORDS`](#\_0x06843DA7060A026B).
---
---**This is the server-side RPC native equivalent of the client native [FREEZE_ENTITY_POSITION](?\_0x428CA6DBD1094446).**
---@param entity integer
---@param toggle boolean
function FreezeEntityPosition(entity, toggle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x62FC38D0)  
---This native does not have an official description.
---@param playerSrc string
---@return number
function GetAirDragMultiplierForPlayersVehicle(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6886C3FE)  
---Returns all object handles known to the server.
---The data returned adheres to the following layout:
---
---```
---[127, 42, 13, 37]
---```
---@return table
function GetAllObjects() end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB8584FEF)  
---Returns all peds handles known to the server.
---The data returned adheres to the following layout:
---
---```
---[127, 42, 13, 37]
---```
---@return table
function GetAllPeds() end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x332169F5)  
---Returns all vehicle handles known to the server.
---The data returned adheres to the following layout:
---
---```
---[127, 42, 13, 37]
---```
---@return table
function GetAllVehicles() end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE57429FA)  
---Returns the current console output buffer.
---@return string
function GetConsoleBuffer() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6CCD2564)  
---Can be used to get a console variable of type `char*`, for example a string.
---@param varName string
---@param default_ string
---@return string
function GetConvar(varName, default_) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7E8EBFE5)  
---Can be used to get a console variable casted back to `bool`.
---@param varName string
---@param defaultValue boolean
---@return boolean
function GetConvarBool(varName, defaultValue) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9E666D)  
---This will have floating point inaccuracy.
---@param varName string
---@param defaultValue number
---@return number
function GetConvarFloat(varName, defaultValue) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x935C0AB2)  
---Can be used to get a console variable casted back to `int` (an integer value).
---@param varName string
---@param default_ integer
---@return integer
function GetConvarInt(varName, default_) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB0237302)  
---Returns the hash of weapon the Ped is currently using.
---@param ped integer
---@return integer
function GetCurrentPedWeapon(ped) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE5E9EBBB)  
---Returns the name of the currently executing resource.
---@return string
function GetCurrentResourceName() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDFFBA12F)  
---### Supported types
---
---*   \[1] : Peds (including animals) and players.
---*   \[2] : Vehicles.
---*   \[3] : Objects (props), doors, and projectiles.
---
---### Coordinates need to be send unpacked (x,y,z)
---
---```lua
---
----- Define the allowed model hashes
---local allowedModelHashes = { GetHashKey("p_crate03x"), GetHashKey("p_crate22x") }
---
----- Get the player's current coordinates
---local playerCoords = GetEntityCoords(PlayerPedId())
---
----- Retrieve all entities of type Object (type 3) within a radius of 10.0 units
----- that match the allowed model hashes
----- and sort output entities by distance
---local entities = GetEntitiesInRadius(playerCoords.x, playerCoords.y, playerCoords.z, 10.0, 3, true, allowedModelHashes)
---
----- Iterate through the list of entities and print their ids
---for i = 1, #entities do
---    local entity = entities[i]
---    print(entity)
---end
---
---```
---@param x number
---@param y number
---@param z number
---@param radius number
---@param entityType integer
---@param sortByDistance boolean
---@param models table
---@return table
function GetEntitiesInRadius(x, y, z, radius, entityType, sortByDistance, models) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFE1589F9)  
---Gets the entity that this entity is attached to.
---@param entity integer
---@return integer
function GetEntityAttachedTo(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE8C0C629)  
---This native does not have an official description.
---@param entity integer
---@return boolean
function GetEntityCollisionDisabled(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1647F1CB)  
---Gets the current coordinates for a specified entity. This native is used server side when using OneSync.
---
---See [GET_ENTITY_COORDS](#\_0x3FEF770D40960D5A) for client side.
---@param entity integer
---@return vector3
function GetEntityCoords(entity) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4BDF1867)  
---Returns the entity handle for the specified state bag name. For use with [ADD_STATE_BAG_CHANGE_HANDLER](#\_0x5BA35AAF).
---@param bagName string
---@return integer
function GetEntityFromStateBagName(bagName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x972CC383)  
---This native does not have an official description.
---@param entity integer
---@return number
function GetEntityHeading(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8E3222B7)  
---Only works for vehicle and peds
---@param entity integer
---@return integer
function GetEntityHealth(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC7AE6AA1)  
---Currently it only works with peds.
---@param entity integer
---@return integer
function GetEntityMaxHealth(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDAFCB3EC)  
---This native does not have an official description.
---@param entity integer
---@return integer
function GetEntityModel(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD16EA02F)  
---This native does not have an official description.
---@param entity integer
---@return integer
function GetEntityOrphanMode(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFC30DDFF)  
---This native gets an entity's population type.
---@param entity integer
---@return integer
function GetEntityPopulationType(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x91B38FB6)  
---This native does not have an official description.
---@param entity integer
---@return boolean
function GetEntityRemoteSyncedScenesAllowed(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8FF45B04)  
---This native does not have an official description.
---@param entity integer
---@return vector3
function GetEntityRotation(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9BF8A73F)  
---This native does not have an official description.
---@param entity integer
---@return vector3
function GetEntityRotationVelocity(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xED4B0486)  
---Gets the routing bucket for the specified entity.
---
---Routing buckets are also known as 'dimensions' or 'virtual worlds' in past echoes, however they are population-aware.
---@param entity integer
---@return integer
function GetEntityRoutingBucket(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB7F70784)  
---This native does not have an official description.
---@param entity integer
---@return string
function GetEntityScript(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9E1E4798)  
---Gets the current speed of the entity in meters per second.
---
---```
---To convert to MPH: speed * 2.236936
---To convert to KPH: speed * 3.6
---```
---@param entity integer
---@return number
function GetEntitySpeed(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB1BD08D)  
---Gets the entity type (as an integer), which can be one of the following defined down below:
---
---**The following entities will return type `1`:**
---
---*   Ped
---*   Player
---*   Animal (Red Dead Redemption 2)
---*   Horse (Red Dead Redemption 2)
---
---**The following entities will return type `2`:**
---
---*   Automobile
---*   Bike
---*   Boat
---*   Heli
---*   Plane
---*   Submarine
---*   Trailer
---*   Train
---*   DraftVeh (Red Dead Redemption 2)
---
---**The following entities will return type `3`:**
---
---*   Object
---*   Door
---*   Pickup
---
---Otherwise, a value of `0` will be returned.
---@param entity integer
---@return integer
function GetEntityType(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC14C9B6B)  
---This native does not have an official description.
---@param entity integer
---@return vector3
function GetEntityVelocity(entity) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x804B9F7B)  
---Returns the internal build number of the current game being executed.
---
---Possible values:
---
---*   FiveM
---    *   1604
---    *   2060
---    *   2189
---    *   2372
---    *   2545
---    *   2612
---    *   2699
---    *   2802
---    *   2944
---    *   3095
---    *   3258
---    *   3323
---    *   3407
---    *   3570
---*   RedM
---    *   1311
---    *   1355
---    *   1436
---    *   1491
---*   LibertyM
---    *   43
---*   FXServer
---    *   0
---@return integer
function GetGameBuildNumber() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE8EAA18B)  
---Returns the current game being executed.
---
---Possible values:
---
---| Return value | Meaning                        |
---| ------------ | ------------------------------ |
---| `fxserver`   | Server-side code ('Duplicity') |
---| `fivem`      | FiveM for GTA V                |
---| `libertym`   | LibertyM for GTA IV            |
---| `redm`       | RedM for Red Dead Redemption 2 |
---@return string
function GetGameName() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2B9D4F50)  
---Returns a list of entity handles (script GUID) for all entities in the specified pool - the data returned is an array as
---follows:
---
---```json
---[ 770, 1026, 1282, 1538, 1794, 2050, 2306, 2562, 2818, 3074, 3330, 3586, 3842, 4098, 4354, 4610, ...]
---```
---
---### Supported pools
---
---*   `CPed`: Peds (including animals) and players.
---*   `CObject`: Objects (props), doors, and projectiles.
---*   `CNetObject`: Networked objects
---*   `CVehicle`: Vehicles.
---*   `CPickup`: Pickups.
---@param poolName string
---@return table
function GetGamePool(poolName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA4EA0691)  
---Gets the current game timer in milliseconds.
---@return integer
function GetGameTimer() end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x98EFF6F1)  
---This native converts the passed string to a hash.
---@param model string
---@return integer
function GetHashKey(model) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA886495D)  
---**Note** This native will always return `1000.0` unless [SET_VEHICLE_BODY_HEALTH](#\_0xB77D05AC8C78AADB), [SET_VEHICLE_ENGINE_HEALTH](#\_0x45F6D8EEF34ABEF1), or [SET_VEHICLE_PETROL_TANK_HEALTH](#\_0x70DB57649FA8D0D8) have been called with a value greater than `1000.0`.
---@param heli integer
---@return integer
function GetHeliBodyHealth(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x82AFC0A3)  
---This is a getter for [SET_DISABLE_HELI_EXPLODE_FROM_BODY_DAMAGE](#\_0xEDBC8405B3895CC9)
---@param heli integer
---@return boolean
function GetHeliDisableExplodeFromBodyDamage(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA0FA0354)  
---**Note** This native will always return `1000.0` unless [SET_VEHICLE_BODY_HEALTH](#\_0xB77D05AC8C78AADB), [SET_VEHICLE_ENGINE_HEALTH](#\_0x45F6D8EEF34ABEF1), or [SET_VEHICLE_PETROL_TANK_HEALTH](#\_0x70DB57649FA8D0D8) have been called with a value greater than `1000.0`.
---@param heli integer
---@return integer
function GetHeliEngineHealth(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD4EC7858)  
---**Note** This native will always return `1000.0` unless [SET_VEHICLE_BODY_HEALTH](#\_0xB77D05AC8C78AADB), [SET_VEHICLE_ENGINE_HEALTH](#\_0x45F6D8EEF34ABEF1), or [SET_VEHICLE_PETROL_TANK_HEALTH](#\_0x70DB57649FA8D0D8) have been called with a value greater than `1000.0`.
---@param heli integer
---@return integer
function GetHeliGasTankHealth(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC37D668)  
---This native does not have an official description.
---@param heli integer
---@return number
function GetHeliMainRotorDamageScale(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF01E2AAB)  
---This native does not have an official description.
---@param vehicle integer
---@return number
function GetHeliMainRotorHealth(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1944AC95)  
---This native does not have an official description.
---@param heli integer
---@return number
function GetHeliPitchControl(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC40161E2)  
---This native does not have an official description.
---@param heli integer
---@return number
function GetHeliRearRotorDamageScale(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x33EE6E2B)  
---This native is a getter for [SET_HELI_TAIL_ROTOR_HEALTH](#\_0xFE205F38AAA58E5B)
---@param vehicle integer
---@return number
function GetHeliRearRotorHealth(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x12948DE9)  
---This native does not have an official description.
---@param heli integer
---@return number
function GetHeliRollControl(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x22239130)  
---This native does not have an official description.
---@param heli integer
---@return number
function GetHeliTailRotorDamageScale(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA41BC13D)  
---**Note**: This native is deprecated, please use [`GET_HELI_REAR_ROTOR_HEALTH`](#\_0x33EE6E2B) instead.
---@param vehicle integer
---@return number
function GetHeliTailRotorHealth(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8E86238D)  
---This native does not have an official description.
---@param heli integer
---@return number
function GetHeliThrottleControl(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8FDC0768)  
---This native does not have an official description.
---@param heli integer
---@return number
function GetHeliYawControl(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x5F70F5A3)  
---This native does not have an official description.
---@return string
function GetHostId() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9F1C4383)  
---This native does not have an official description.
---@return integer
function GetInstanceId() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4D52FE5B)  
---This native does not have an official description.
---@return string
function GetInvokingResource() end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x3EFE38D1)  
---This native does not have an official description.
---@param heli integer
---@return boolean
function GetIsHeliEngineRunning(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7DC6D022)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function GetIsVehicleEngineRunning(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD7EC8760)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function GetIsVehiclePrimaryColourCustom(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x288AD228)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function GetIsVehicleSecondaryColourCustom(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA6F02670)  
---See the client-side [GET_LANDING_GEAR_STATE](#\_0x9B0F3DCA3DB0F4CD) native for a description of landing gear states.
---@param vehicle integer
---@return integer
function GetLandingGearState(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF7C6792D)  
---This native does not have an official description.
---@param vehicle integer
---@param seatIndex integer
---@return integer
function GetLastPedInVehicleSeat(vehicle, seatIndex) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x23B2A641)  
---Gets the specific entity type (as an integer), which can be one of the following defined down below:
---
---#### FiveM:
---
---```cpp
---enum eNetObjEntityType
---{
---    Automobile = 0,
---    Bike = 1,
---    Boat = 2,
---    Door = 3,
---    Heli = 4,
---    Object = 5,
---    Ped = 6,
---    Pickup = 7,
---    PickupPlacement = 8,
---    Plane = 9,
---    Submarine = 10,
---    Player = 11,
---    Trailer = 12,
---    Train = 13
---};
---```
---
---#### RedM:
---
---```cpp
---enum eNetObjEntityType
---{
---    Animal = 0,
---    Automobile = 1,
---    Bike = 2,
---    Boat = 3,
---    Door = 4,
---    Heli = 5,
---    Object = 6,
---    Ped = 7,
---    Pickup = 8,
---    PickupPlacement = 9,
---    Plane = 10,
---    Submarine = 11,
---    Player = 12,
---    Trailer = 13,
---    Train = 14,
---    DraftVeh = 15,
---    StatsTracker = 16,
---    PropSet = 17,
---    AnimScene = 18,
---    GroupScenario = 19,
---    Herd = 20,
---    Horse = 21,
---    WorldState = 22,
---    WorldProjectile = 23,
---    Incident = 24,
---    Guardzone = 25,
---    PedGroup = 26,
---    CombatDirector = 27,
---    PedSharedTargeting = 28,
---    Persistent = 29
---};
---```
---@param entity integer
---@return integer
function GetNetTypeFromEntity(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFF7F66AB)  
---This native does not have an official description.
---@param playerSrc string
---@return integer
function GetNumPlayerIdentifiers(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x63D13184)  
---This native does not have an official description.
---@return integer
function GetNumPlayerIndices() end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x619E4A3D)  
---This native does not have an official description.
---@param playerSrc string
---@return integer
function GetNumPlayerTokens(playerSrc) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x776E864)  
---Gets the amount of metadata values with the specified key existing in the specified resource's manifest.
---See also: [Resource manifest](https://docs.fivem.net/docs/scripting-reference/resource-manifest/resource-manifest/)
---@param resourceName string
---@param metadataKey string
---@return integer
function GetNumResourceMetadata(resourceName, metadataKey) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x863F27B)  
---This native does not have an official description.
---@return integer
function GetNumResources() end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x23473EA4)  
---This native does not have an official description.
---@param password string
---@return string
function GetPasswordHash(password) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2CE311A7)  
---This native does not have an official description.
---@param ped integer
---@return integer
function GetPedArmour(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x63458C27)  
---This native does not have an official description.
---@param ped integer
---@return integer
function GetPedCauseOfDeath(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC182F76E)  
---This native does not have an official description.
---@param ped integer
---@return number
function GetPedDesiredHeading(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x388FDE9A)  
---This native does not have an official description.
---@param vehicle integer
---@param seatIndex integer
---@return integer
function GetPedInVehicleSeat(vehicle, seatIndex) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA45B6C8D)  
---This native does not have an official description.
---@param ped integer
---@return integer
function GetPedMaxHealth(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x354F283C)  
---Gets the current relationship group hash of a ped.
---@param ped integer
---@return integer
function GetPedRelationshipGroupHash(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x84FE084)  
---Gets the script task command currently assigned to the ped.
---@param ped integer
---@return integer
function GetPedScriptTaskCommand(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x44B0E5E2)  
---Gets the stage of the peds scripted task.
---@param ped integer
---@return integer
function GetPedScriptTaskStage(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x535DB43F)  
---Get the last entity that damaged the ped. This native is used server side when using OneSync.
---@param ped integer
---@return integer
function GetPedSourceOfDamage(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x84ADF9EB)  
---Get the entity that killed the ped. This native is used server side when using OneSync.
---@param ped integer
---@return integer
function GetPedSourceOfDeath(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7F4563D3)  
---Gets the type of a ped's specific task given an index of the CPedTaskSpecificDataNode nodes.
---A ped will typically have a task at index 0, if a ped has multiple tasks at once they will be in the order 0, 1, 2, etc.
---@param ped integer
---@param index integer
---@return integer
function GetPedSpecificTaskType(ped, index) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x40321B83)  
---This native does not have an official description.
---@param ped integer
---@return boolean
function GetPedStealthMovement(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x433C765D)  
---Gets the current camera rotation for a specified player. This native is used server side when using OneSync.
---@param playerSrc string
---@return vector3
function GetPlayerCameraRotation(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFEE404F9)  
---This native does not have an official description.
---@param playerSrc string
---@return string
function GetPlayerEndpoint(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x98D244)  
---Gets the current fake wanted level for a specified player. This native is used server side when using OneSync.
---@param playerSrc string
---@return integer
function GetPlayerFakeWantedLevel(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x586F80FF)  
---Gets the focus position (i.e. the position of the active camera in the game world) of a player.
---@param playerSrc string
---@return vector3
function GetPlayerFocusPos(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC8A9CE08)  
---This native does not have an official description.
---@param index integer
---@return string
function GetPlayerFromIndex(index) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA56135E0)  
---On the server this will return the players source, on the client it will return the player handle.
---@param bagName string
---@return integer
function GetPlayerFromStateBagName(bagName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE52D9680)  
---This native does not have an official description.
---@param playerSrc string
---@return string
function GetPlayerGuid(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7302DBCF)  
---To get the number of identifiers, use [GET_NUM_PLAYER_IDENTIFIERS](#\_0xFF7F66AB)
---
---To get a specific type of identifier, use [GET_PLAYER_IDENTIFIER_BY_TYPE](#\_0xA61C8FC6)
---@param playerSrc string
---@param identiferIndex integer
---@return string
function GetPlayerIdentifier(playerSrc, identiferIndex) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA61C8FC6)  
---Get an identifier from a player by the type of the identifier.
---Known [Identifiers](https://docs.fivem.net/docs/scripting-reference/runtimes/lua/functions/GetPlayerIdentifiers/#identifier-types)
---@param playerSrc string
---@param identifierType string
---@return string
function GetPlayerIdentifierByType(playerSrc, identifierType) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x680C90EE)  
---This native does not have an official description.
---@param playerSrc string
---@return boolean
function GetPlayerInvincible(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x427E8E6A)  
---This native does not have an official description.
---@param playerSrc string
---@return integer
function GetPlayerLastMsg(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2A50657)  
---This native does not have an official description.
---@param playerSrc string
---@return integer
function GetPlayerMaxArmour(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8154E470)  
---This native does not have an official description.
---@param playerSrc string
---@return integer
function GetPlayerMaxHealth(playerSrc) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8689A825)  
---A getter for [SET_PLAYER_MELEE_WEAPON_DAMAGE_MODIFIER](#\_0x4A3DC7ECCC321032).
---@param playerId integer
---@return number
function GetPlayerMeleeWeaponDamageModifier(playerId) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x406B4B20)  
---This native does not have an official description.
---@param playerSrc string
---@return string
function GetPlayerName(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6E31E993)  
---Used to get the player's Ped Entity ID when a valid `playerSrc` is passed.
---@param playerSrc string
---@return integer
function GetPlayerPed(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9A928294)  
---```cpp
---const int ENET_PACKET_LOSS_SCALE = 65536;
---
---enum PeerStatistics
---{
---	// PacketLoss will only update once every 10 seconds, use PacketLossEpoch if you want the time
---	// since the last time the packet loss was updated.
---
---	// the amount of packet loss the player has, needs to be scaled with PACKET_LOSS_SCALE
---	PacketLoss = 0,
---	// The variance in the packet loss
---	PacketLossVariance = 1,
---	// The time since the last packet update in ms, relative to the peers connection time
---	PacketLossEpoch = 2,
---	// The mean amount of time it takes for a packet to get to the client (ping)
---	RoundTripTime = 3,
---	// The variance in the round trip time
---	RoundTripTimeVariance = 4,
---	// Despite their name, these are only updated once every 5 seconds, you can get the last time this was updated with PacketThrottleEpoch
---	// The last recorded round trip time of a packet
---	LastRoundTripTime = 5,
---	// The last round trip time variance
---	LastRoundTripTimeVariance = 6,
---	// The time since the last packet throttle update, relative to the peers connection time
---	PacketThrottleEpoch = 7,
---};
---```
---
---These statistics only update once every 10 seconds.
---@param playerSrc string
---@param peerStatistic integer
---@return integer
function GetPlayerPeerStatistics(playerSrc, peerStatistic) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFF1290D4)  
---See [GET_PLAYER_PEER_STATISTICS](#\_0x9A928294) if you want more detailed information, like packet loss, and packet/rtt variance
---@param playerSrc string
---@return integer
function GetPlayerPing(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x52441C34)  
---Gets the routing bucket for the specified player.
---
---Routing buckets are also known as 'dimensions' or 'virtual worlds' in past echoes, however they are population-aware.
---@param playerSrc string
---@return integer
function GetPlayerRoutingBucket(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9873E404)  
---This native does not have an official description.
---@param playerSrc string
---@return integer
function GetPlayerTeam(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7ADE63E1)  
---```
---Gets the amount of time player has spent evading the cops.
---Counter starts and increments only when cops are chasing the player.
---If the player is evading, the timer will pause.
---```
---@param playerSrc string
---@param lastPursuit boolean
---@return integer
function GetPlayerTimeInPursuit(playerSrc, lastPursuit) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x67D2E605)  
---Gets the current time online for a specified player.
---@param playerSrc string
---@return integer
function GetPlayerTimeOnline(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x54C06897)  
---Gets a player's token. Tokens can be used to enhance banning logic, however are specific to a server.
---@param playerSrc string
---@param index integer
---@return string
function GetPlayerToken(playerSrc, index) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x821F2D2C)  
---Gets the current known coordinates for the specified player from cops perspective. This native is used server side when using OneSync.
---@param playerSrc string
---@return vector3
function GetPlayerWantedCentrePosition(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xBDCDD163)  
---```
---Returns given players wanted level server-side.
---```
---@param playerSrc string
---@return integer
function GetPlayerWantedLevel(playerSrc) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2A3D7CDA)  
---A getter for [SET_PLAYER_WEAPON_DAMAGE_MODIFIER](#\_0xCE07B9F7817AADA3).
---@param playerId integer
---@return number
function GetPlayerWeaponDamageModifier(playerId) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF1543251)  
---A getter for [SET_PLAYER_WEAPON_DEFENSE_MODIFIER](#\_0x2D83BC011CA14A3C).
---@param playerId integer
---@return number
function GetPlayerWeaponDefenseModifier(playerId) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x986B65FF)  
---A getter for [\_SET_PLAYER_WEAPON_DEFENSE_MODIFIER\_2](#\_0xBCFDE9EDE4CF27DC).
---@param playerId integer
---@return number
function GetPlayerWeaponDefenseModifier2(playerId) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD4BEF069)  
---Returns all commands that are registered in the command system.
---The data returned adheres to the following layout:
---
---```
---[
---{
---"name": "cmdlist",
---"resource": "resource",
---"arity" = -1,
---},
---{
---"name": "command1"
---"resource": "resource_2",
---"arity" = -1,
---}
---]
---```
---@return table
function GetRegisteredCommands() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x387246B7)  
---This native does not have an official description.
---@param findIndex integer
---@return string
function GetResourceByFindIndex(findIndex) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x97628584)  
---Returns all commands registered by the specified resource.
---The data returned adheres to the following layout:
---
---```
---[
---{
---"name": "cmdlist",
---"resource": "example_resource",
---"arity" = -1,
---},
---{
---"name": "command1"
---"resource": "example_resource2",
---"arity" = -1,
---}
---]
---```
---@param resource string
---@return table
function GetResourceCommands(resource) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x35BDCEEA)  
---A getter for [SET_RESOURCE_KVP_FLOAT](#\_0x9ADD2938).
---@param key string
---@return number
function GetResourceKvpFloat(key) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x557B586A)  
---A getter for [SET_RESOURCE_KVP_INT](#\_0x6A2B1E8).
---@param key string
---@return integer
function GetResourceKvpInt(key) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x5240DA5A)  
---A getter for [SET_RESOURCE_KVP](#\_0x21C7A35B).
---@param key string
---@return string
function GetResourceKvpString(key) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x964BAB1D)  
---Gets the metadata value at a specified key/index from a resource's manifest.
---See also: [Resource manifest](https://docs.fivem.net/docs/scripting-reference/resource-manifest/resource-manifest/)
---@param resourceName string
---@param metadataKey string
---@param index integer
---@return string
function GetResourceMetadata(resourceName, metadataKey, index) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x61DCF017)  
---Returns the physical on-disk path of the specified resource.
---@param resourceName string
---@return string
function GetResourcePath(resourceName) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4039B485)  
---Returns the current state of the specified resource.
---@param resourceName string
---@return string
function GetResourceState(resourceName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD240123E)  
---An alias of [GET_CURRENT_PED_WEAPON](#\_0xB0237302).
---
---Note, the client-side [GET_SELECTED_PED_WEAPON](#\_0x0A6DB4965674D243) native returns the weapon selected via the HUD (weapon wheel). This data is not available to FXServer.
---@param ped integer
---@return integer
function GetSelectedPedWeapon(ped) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x78D864C7)  
---This native does not have an official description.
---@param bagName string
---@return table
function GetStateBagKeys(bagName) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x637F4C75)  
---Returns the value of a state bag key.
---@param bagName string
---@param key string
---@return table
function GetStateBagValue(bagName, key) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1C939E87)  
---This native does not have an official description.
---@param jetpack integer
---@return number
function GetThrusterSideRcsThrottle(jetpack) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x94E24C96)  
---This native does not have an official description.
---@param jetpack integer
---@return number
function GetThrusterThrottle(jetpack) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x456E34A)  
---This native does not have an official description.
---@param train integer
---@return integer
function GetTrainBackwardCarriage(train) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x95070FA)  
---This native does not have an official description.
---@param train integer
---@return integer
function GetTrainCarriageEngine(train) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4B8285CF)  
---This native does not have an official description.
---@param train integer
---@return integer
function GetTrainCarriageIndex(train) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA4921EF5)  
---Gets the trains desired speed.
---@param train integer
---@return number
function GetTrainCruiseSpeed(train) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8DAF79B6)  
---Gets the direction the train is facing
---@param train integer
---@return boolean
function GetTrainDirection(train) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x24DC88D9)  
---This native does not have an official description.
---@param train integer
---@return integer
function GetTrainForwardCarriage(train) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x81B50033)  
---This native does not have an official description.
---@param train integer
---@return integer
function GetTrainState(train) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9AA339D)  
---This native does not have an official description.
---@param train integer
---@return integer
function GetTrainTrackIndex(train) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2B2FCC28)  
---This native does not have an official description.
---@param vehicle integer
---@return number
function GetVehicleBodyHealth(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x40D82D88)  
---This native does not have an official description.
---@param vehicle integer
---@return integer, integer
function GetVehicleColours(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1C2B9FEF)  
---This native does not have an official description.
---@param vehicle integer
---@return integer, integer, integer
function GetVehicleCustomPrimaryColour(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x3FF247A2)  
---This native does not have an official description.
---@param vehicle integer
---@return integer, integer, integer
function GetVehicleCustomSecondaryColour(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA0DBD08D)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleDashboardColour(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFD15C065)  
---This native does not have an official description.
---@param vehicle integer
---@return number
function GetVehicleDirtLevel(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD72CEF2)  
---```lua
---enum_VehicleLockStatus = {
---    None = 0,
---    Locked = 2,
---    LockedForPlayer = 3,
---    StickPlayerInside = 4, -- Doesn't allow players to exit the vehicle with the exit vehicle key.
---    CanBeBrokenInto = 7, -- Can be broken into the car. If the glass is broken, the value will be set to 1
---    CanBeBrokenIntoPersist = 8, -- Can be broken into persist
---    CannotBeTriedToEnter = 10, -- Cannot be tried to enter (Nothing happens when you press the vehicle enter key).
---}
---```
---
---It should be [noted](https://forum.cfx.re/t/4863241) that while the [client-side command](#\_0x25BC98A59C2EA962) and its
---setter distinguish between states 0 (unset) and 1 (unlocked), the game will synchronize both as state 0, so the server-side
---command will return only '0' if unlocked.
---@param vehicle integer
---@return integer
function GetVehicleDoorLockStatus(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6E35C49C)  
---Returns the open position of the specified door on the target vehicle.
---@param vehicle integer
---@param doorIndex integer
---@return integer
function GetVehicleDoorStatus(vehicle, doorIndex) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1DC50247)  
---Currently it only works when set to "all players".
---@param vehicle integer
---@return integer
function GetVehicleDoorsLockedForPlayer(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8880038A)  
---This native does not have an official description.
---@param vehicle integer
---@return number
function GetVehicleEngineHealth(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x80E4659B)  
---This native does not have an official description.
---@param vehicle integer
---@return integer, integer
function GetVehicleExtraColours(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xAD40AD55)  
---Gets the flight nozzel position for the specified vehicle. See the client-side [\_GET_VEHICLE_FLIGHT_NOZZLE_POSITION](#\_0xDA62027C8BDB326E) native for usage examples.
---@param vehicle integer
---@return number
function GetVehicleFlightNozzlePosition(vehicle) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x483B013C)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function GetVehicleHandbrake(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD7147656)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleHeadlightsColour(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFBDE9FD8)  
---Gets the lock on state for the specified vehicle. See the client-side [GET_VEHICLE_HOMING_LOCKON_STATE](#\_0xE6B0E8CFC3633BF0) native for a description of lock on states.
---@param vehicle integer
---@return integer
function GetVehicleHomingLockonState(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDEA49773)  
---This is a getter for the client-side native [`START_VEHICLE_HORN`](#\_0x9C8C6504B5B63D2C), which allows you to return the horn type of the vehicle.
---
---**Note**: This native only gets the hash value set with `START_VEHICLE_HORN`. If a wrong hash is passed into `START_VEHICLE_HORN`, it will return this wrong hash.
---
---```cpp
---enum eHornTypes
---{
---    NORMAL = 1330140148,
---    HELDDOWN = -2087385909,
---    AGGRESSIVE = -92810745
---}
---```
---@param vehicle integer
---@return integer
function GetVehicleHornType(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xCCFF3B6E)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleInteriorColour(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7C278621)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean, boolean, boolean
function GetVehicleLightsState(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xEC82A51D)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleLivery(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4A557117)  
---Gets the vehicle that is locked on to for the specified vehicle.
---@param vehicle integer
---@return integer
function GetVehicleLockOnTarget(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD9319DCB)  
---Getter to check the neon colour of a vehicle. This native is the server side getter of [GET_VEHICLE_NEON_LIGHTS_COLOUR](#\_0x7619EEE8C886757F).
---@param vehicle integer
---@return integer, integer, integer
function GetVehicleNeonColour(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x684BDBF2)  
---Getter to check if one of the neon lights of a vehicle is enabled. This native is the server side getter of [IS_VEHICLE_NEON_LIGHT_ENABLED](#\_0x8C4B92553E4766A5).
---
---```cpp
---enum neonIndex
---{
---    NEON_BACK = 0,   // Back neon
---    NEON_RIGHT = 1,  // Right neon
---    NEON_LEFT = 2,   // Left neon
---    NEON_FRONT = 3   // Front neon
---};
---```
---@param vehicle integer
---@param neonIndex integer
---@return boolean
function GetVehicleNeonEnabled(vehicle, neonIndex) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE8522D58)  
---This native does not have an official description.
---@param vehicle integer
---@return string
function GetVehicleNumberPlateText(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x499747B6)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleNumberPlateTextIndex(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xAFE92319)  
---Gets the vehicle the specified Ped is/was in depending on bool value. This native is used server side when using OneSync.
---@param ped integer
---@param lastVehicle boolean
---@return integer
function GetVehiclePedIsIn(ped, lastVehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE41595CE)  
---This native does not have an official description.
---@param vehicle integer
---@return number
function GetVehiclePetrolTankHealth(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x57037960)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleRadioStationIndex(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x872CF42)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleRoofLivery(vehicle) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1382FCEA)  
---This native does not have an official description.
---@param vehicle integer
---@return number
function GetVehicleSteeringAngle(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9963D5F9)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleTotalRepairs(vehicle) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA273060E)  
---Returns the type of the passed vehicle.
---
---For client scripts, reference the more detailed [GET_VEHICLE_TYPE_RAW](#\_0xDE73BC10) native.
---
---### Vehicle types
---
---*   automobile
---*   bike
---*   boat
---*   heli
---*   plane
---*   submarine
---*   trailer
---*   train
---@param vehicle integer
---@return string
function GetVehicleType(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x75280015)  
---This native does not have an official description.
---@param vehicle integer
---@return integer, integer, integer
function GetVehicleTyreSmokeColor(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDA58D7AE)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleWheelType(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x13D53892)  
---This native does not have an official description.
---@param vehicle integer
---@return integer
function GetVehicleWindowTint(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x3E1E286D)  
---GIVE_WEAPON_COMPONENT_TO_PED
---
---**This is the server-side RPC native equivalent of the client native [GIVE_WEAPON_COMPONENT_TO_PED](?\_0xD966D51AA5B28BB9).**
---@param ped integer
---@param weaponHash integer | string
---@param componentHash integer | string
function GiveWeaponComponentToPed(ped, weaponHash, componentHash) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC4D88A85)  
---GIVE_WEAPON_TO_PED
---
---**This is the server-side RPC native equivalent of the client native [GIVE_WEAPON_TO_PED](?\_0xBF0FD6E56C964FCB).**
---@param ped integer
---@param weaponHash integer | string
---@param ammoCount integer
---@param isHidden boolean
---@param bForceInHand boolean
function GiveWeaponToPed(ped, weaponHash, ammoCount, isHidden, bForceInHand) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9C9A3BE0)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function HasEntityBeenMarkedAsNoLongerNeeded(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB8AF3137)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function HasVehicleBeenDamagedByBullets(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE4E83A5B)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function HasVehicleBeenOwnedByPlayer(vehicle) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7EBB9929)  
---This native does not have an official description.
---@param object string
---@return boolean
function IsAceAllowed(object) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD5C39EE6)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function IsBoatAnchoredAndFrozen(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9049DB44)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function IsBoatWrecked(vehicle) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xCF24C52E)  
---Gets whether or not this is the CitizenFX server.
---@return boolean
function IsDuplicityVersion() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xEDBE6ADD)  
---A getter for [FREEZE_ENTITY_POSITION](#\_0x428CA6DBD1094446).
---@param entity integer
---@return boolean
function IsEntityPositionFrozen(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x120B4ED5)  
---This native checks if the given entity is visible.
---@param entity integer
---@return boolean
function IsEntityVisible(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x76876154)  
---This native does not have an official description.
---@param ped integer
---@return boolean
function IsFlashLightOn(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x23E46BD7)  
---This is a getter for [SET_HELI_TAIL_EXPLODE_THROW_DASHBOARD](#\_0x3EC8BF18AA453FE9)
---@param heli integer
---@return boolean
function IsHeliTailBoomBreakable(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2C59F987)  
---This native does not have an official description.
---@param heli integer
---@return boolean
function IsHeliTailBoomBroken(heli) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x404794CA)  
---This native checks if the given ped is a player.
---@param ped integer
---@return boolean
function IsPedAPlayer(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x25865633)  
---This native does not have an official description.
---@param ped integer
---@return boolean
function IsPedHandcuffed(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC833BBE1)  
---This native does not have an official description.
---@param ped integer
---@return boolean
function IsPedRagdoll(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xEFEED13C)  
---This native does not have an official description.
---@param ped integer
---@return boolean
function IsPedStrafing(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x5AE7EDA2)  
---This native does not have an official description.
---@param ped integer
---@return boolean
function IsPedUsingActionMode(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDEDAE23D)  
---This native does not have an official description.
---@param playerSrc string
---@param object string
---@return boolean
function IsPlayerAceAllowed(playerSrc, object) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xBEFE93F4)  
---Requests whether or not the commerce data for the specified player has loaded.
---@param playerSrc string
---@return boolean
function IsPlayerCommerceInfoLoaded(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1D14F4FE)  
---Requests whether or not the commerce data for the specified player has loaded from Tebex.
---@param playerSrc string
---@return boolean
function IsPlayerCommerceInfoLoadedExt(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x89A3881A)  
---```
---This will return true if the player is evading wanted level, meaning that the wanted level stars are blink.
---Otherwise will return false.
---
---If the player is not wanted, it simply returns false.
---```
---@param playerSrc string
---@return boolean
function IsPlayerEvadingWantedLevel(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1F14F2AC)  
---This native does not have an official description.
---@param playerSrc string
---@return boolean
function IsPlayerInFreeCamMode(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC7D2C20C)  
---This native does not have an official description.
---@param playerSrc string
---@return boolean
function IsPlayerUsingSuperJump(playerSrc) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x37CF52CE)  
---This native does not have an official description.
---@param principal string
---@param object string
---@return boolean
function IsPrincipalAceAllowed(principal, object) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFA9336E5)  
---This native does not have an official description.
---@param train integer
---@return boolean
function IsTrainCaboose(train) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xBB340D04)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function IsVehicleEngineStarting(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x42098B5)  
---This native does not have an official description.
---@param vehicle integer
---@param extraId integer
---@return boolean
function IsVehicleExtraTurnedOn(vehicle, extraId) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x25EB5873)  
---This native does not have an official description.
---@param vehicle integer
---@return boolean
function IsVehicleSirenOn(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x48C80210)  
---This native does not have an official description.
---@param vehicle integer
---@param wheelID integer
---@param completely boolean
---@return boolean
function IsVehicleTyreBurst(vehicle, wheelID, completely) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xAC4EF23D)  
---See the client-side [IS_VEHICLE_WINDOW_INTACT](#\_0x46E571A0E20D01F1) for a window indexes list.
---@param vehicle integer
---@param windowIndex integer
---@return boolean
function IsVehicleWindowIntact(vehicle, windowIndex) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA8F63EAB)  
---Requests the commerce data for the specified player, including the owned SKUs. Use `IS_PLAYER_COMMERCE_INFO_LOADED` to check if it has loaded.
---@param playerSrc string
function LoadPlayerCommerceData(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7995539E)  
---Requests the commerce data from Tebex for the specified player, including the owned SKUs.
---
---Use [`IS_PLAYER_COMMERCE_INFO_LOADED_EXT`](#\_0x1D14F4FE) to check if it has loaded.
---
---This will not automatically update whenever a client purchases a package, if you want to fetch new purchases you will need to call this native again.
---
---This native will temporarily cache the players commerce data for 10 seconds, a call to this native after 10 seconds will re-fetch the players commerce data.
---@param playerSrc string
function LoadPlayerCommerceDataExt(playerSrc) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x76A9EE1F)  
---Reads the contents of a text file in a specified resource.
---If executed on the client, this file has to be included in `files` in the resource manifest.
---Example: `local data = LoadResourceFile("devtools", "data.json")`
---@param resourceName string
---@param fileName string
---@return string
function LoadResourceFile(resourceName, fileName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x262663C5)  
---Create a permanent voice channel.
---@param id integer
function MumbleCreateChannel(id) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1D5D50C2)  
---Checks if the player is currently muted
---@param playerSrc integer
---@return boolean
function MumbleIsPlayerMuted(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xCC6C2EB1)  
---Mutes or unmutes the specified player
---@param playerSrc integer
---@param toggle boolean
function MumbleSetPlayerMuted(playerSrc, toggle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x5B912C3F)  
---This native does not have an official description.
---@param netId integer
---@return integer
function NetworkGetEntityFromNetworkId(netId) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x526FEE31)  
---Returns the owner ID of the specified entity.
---@param entity integer
---@return integer
function NetworkGetEntityOwner(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1E546224)  
---Returns the first owner ID of the specified entity.
---@param entity integer
---@return integer
function NetworkGetFirstEntityOwner(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9E35DAB6)  
---This native does not have an official description.
---@param entity integer
---@return integer
function NetworkGetNetworkIdFromEntity(entity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFFEEF513)  
---This native does not have an official description.
---@param playerSrc string
---@return vector3
function NetworkGetVoiceProximityOverrideForPlayer(playerSrc) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8E8CC653)  
---This native does not have an official description.
---@param requestData string
---@param requestDataLength integer
---@return integer
function PerformHttpRequestInternal(requestData, requestDataLength) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6B171E87)  
---This native does not have an official description.
---@param requestData table
---@return integer
function PerformHttpRequestInternalEx(requestData) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x90892DED)  
---Prints 'structured trace' data to the server `file descriptor 3` channel. This is not generally useful outside of
---server monitoring utilities.
---@param jsonString string
function PrintStructuredTrace(jsonString) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC795A4A9)  
---Scope entry for profiler.
---@param scopeName string
function ProfilerEnterScope(scopeName) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB39CA35C)  
---Scope exit for profiler.
function ProfilerExitScope() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF8B7D7BB)  
---Returns true if the profiler is active.
---@return boolean
function ProfilerIsRecording() end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x5FA79B0F)  
---Registered commands can be executed by entering them in the client console (this works for client side and server side registered commands). Or by entering them in the server console/through an RCON client (only works for server side registered commands). Or if you use a supported chat resource, like the default one provided in the cfx-server-data repository, then you can enter the command in chat by prefixing it with a `/`.
---
---Commands registered using this function can also be executed by resources, using the [`ExecuteCommand` native](#\_0x561C060B).
---
---The restricted bool is not used on the client side. Permissions can only be checked on the server side, so if you want to limit your command with an ace permission automatically, make it a server command (by registering it in a server script).
---
---**Example result**:
---
---![](https://i.imgur.com/TaCnG09.png)
---@param commandName string
---@param handler function
---@param restricted boolean
function RegisterCommand(commandName, handler, restricted) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x281B5448)  
---Registers a listener for console output messages.
---@param listener function
function RegisterConsoleListener(listener) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD233A168)  
---An internal function which allows the current resource's HLL script runtimes to receive state for the specified event.
---@param eventName string
function RegisterResourceAsEventHandler(eventName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9862B266)  
---**Experimental**: This native may be altered or removed in future versions of CitizenFX without warning.
---
---Registers a cached resource asset with the resource system, similar to the automatic scanning of the `stream/` folder.
---@param resourceName string
---@param fileName string
---@return string
function RegisterResourceAsset(resourceName, fileName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x285B43CA)  
---Registers a build task factory for resources.
---The function should return an object (msgpack map) with the following fields:
---
---```
---{
---// returns whether the specific resource should be built
---shouldBuild = func(resourceName: string): bool,
---
---// asynchronously start building the specific resource.
---// call cb when completed
---build = func(resourceName: string, cb: func(success: bool, status: string): void): void
---}
---```
---@param factoryId string
---@param factoryFn function
function RegisterResourceBuildTaskFactory(factoryId, factoryFn) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA44CE817)  
---Parameter `p1` does not seem to be used or referenced in game binaries.\
---**Note:** When called for networked entities, a `CRemoveAllWeaponsEvent` will be created per request.
---
---**This is the server-side RPC native equivalent of the client native [REMOVE_ALL_PED_WEAPONS](?\_0xF25DF915FA38C5F3).**
---@param ped integer
---@param p1 boolean
function RemoveAllPedWeapons(ped, p1) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD8C3C1CD)  
---Removes the blip from your map.
---**Note:** This function only works on the script that created the blip, if you wish to remove blips created by other scripts, see [`SET_THIS_SCRIPT_CAN_REMOVE_BLIPS_CREATED_BY_ANY_SCRIPT`](#\_0x86A652570E5F25DD).
---
---**This is the server-side RPC native equivalent of the client native [REMOVE_BLIP](?\_0x86A652570E5F25DD).**
---@param blip integer
function RemoveBlip(blip) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xEAC49841)  
---This native does not have an official description.
---@param cookie integer
function RemoveConvarChangeListener(cookie) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD36BE661)  
---**Experimental**: This native may be altered or removed in future versions of CitizenFX without warning.
---
---Removes a handler for changes to a state bag.
---@param cookie integer
function RemoveStateBagChangeHandler(cookie) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x412AA00D)  
---REMOVE_WEAPON_COMPONENT_FROM_PED
---
---**This is the server-side RPC native equivalent of the client native [REMOVE_WEAPON_COMPONENT_FROM_PED](?\_0x1E8BE90C74FB4C09).**
---@param ped integer
---@param weaponHash integer | string
---@param componentHash integer | string
function RemoveWeaponComponentFromPed(ped, weaponHash, componentHash) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9C37F220)  
---```
---This native removes a specified weapon from your selected ped.
---Weapon Hashes: pastebin.com/0wwDZgkF
---Example:
---C#:
---Function.Call(Hash.REMOVE_WEAPON_FROM_PED, Game.Player.Character, 0x99B507EA);
---C++:
---WEAPON::REMOVE_WEAPON_FROM_PED(PLAYER::PLAYER_PED_ID(), 0x99B507EA);
---The code above removes the knife from the player.
---```
---
---**This is the server-side RPC native equivalent of the client native [REMOVE_WEAPON_FROM_PED](?\_0x4899CB088EDF59B8).**
---@param ped integer
---@param weaponHash integer | string
function RemoveWeaponFromPed(ped, weaponHash) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x96F93CCE)  
---Requests the specified player to buy the passed SKU. This'll pop up a prompt on the client, which upon acceptance
---will open the browser prompting further purchase details.
---@param playerSrc string
---@param skuId integer
function RequestPlayerCommerceSession(playerSrc, skuId) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA09E7E7B)  
---Writes the specified data to a file in the specified resource.
---Using a length of `-1` will automatically detect the length assuming the data is a C string.
---@param resourceName string
---@param fileName string
---@param data string
---@param dataLength integer
---@return boolean
function SaveResourceFile(resourceName, fileName, data, dataLength) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x636F097F)  
---Scans the resources in the specified resource root. This function is only available in the 'monitor mode' process and is
---not available for user resources.
---@param rootPath string
---@param callback function
function ScanResourceRoot(rootPath, callback) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB88A73AD)  
---Schedules the specified resource to run a tick as soon as possible, bypassing the server's fixed tick rate.
---@param resourceName string
function ScheduleResourceTick(resourceName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8DBBB0B9)  
---Sets the displayed sprite for a specific blip.
---There's a [list of sprites](https://docs.fivem.net/game-references/blips/) on the FiveM documentation site.
---
---**This is the server-side RPC native equivalent of the client native [SET_BLIP_SPRITE](?\_0xDF735600A4696DAF).**
---@param blip integer
---@param spriteId integer
function SetBlipSprite(blip, spriteId) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x341B16D2)  
---This native does not have an official description.
---@param varName string
---@param value string
function SetConvar(varName, value) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF292858C)  
---Used to replicate a server variable onto clients.
---@param varName string
---@param value string
function SetConvarReplicated(varName, value) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9338D547)  
---This native does not have an official description.
---@param varName string
---@param value string
function SetConvarServerInfo(varName, value) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB8278882)  
---SET_CURRENT_PED_WEAPON
---
---**This is the server-side RPC native equivalent of the client native [SET_CURRENT_PED_WEAPON](?\_0xADF692B254977C0C).**
---@param ped integer
---@param weaponHash integer | string
---@param bForceInHand boolean
function SetCurrentPedWeapon(ped, weaponHash, bForceInHand) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDF70B41B)  
---Sets the coordinates (world position) for a specified entity, offset by the radius of the entity on the Z axis.
---
---**This is the server-side RPC native equivalent of the client native [SET_ENTITY_COORDS](?\_0x06843DA7060A026B).**
---@param entity integer
---@param xPos number
---@param yPos number
---@param zPos number
---@param alive boolean
---@param deadFlag boolean
---@param ragdollFlag boolean
---@param clearArea boolean
function SetEntityCoords(entity, xPos, yPos, zPos, alive, deadFlag, ragdollFlag, clearArea) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD3A183A3)  
---It overrides the default distance culling radius of an entity. Set to `0.0` to reset.
---If you want to interact with an entity outside of your players' scopes set the radius to a huge number.
---
---**WARNING**: Culling natives are deprecated and have known, [unfixable issues](https://forum.cfx.re/t/issue-with-culling-radius-and-server-side-entities/4900677/4)
---@param entity integer
---@param radius number
function SetEntityDistanceCullingRadius(entity, radius) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE0FF064D)  
---Set the heading of an entity in degrees also known as "Yaw".
---
---**This is the server-side RPC native equivalent of the client native [SET_ENTITY_HEADING](?\_0x8E2530AA8ADA980E).**
---@param entity integer
---@param heading number
function SetEntityHeading(entity, heading) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9F7F8D36)  
---It allows to flag an entity to ignore the request control filter policy.
---@param entity integer
---@param ignore boolean
function SetEntityIgnoreRequestControlFilter(entity, ignore) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x489E9162)  
---```cpp
---enum EntityOrphanMode {
---    // Default, this will delete the entity when it isn't relevant to any players
---    // NOTE: this *doesn't* mean when they're no longer in scope
---    DeleteWhenNotRelevant = 0,
---    // The entity will be deleted whenever its original owner disconnects
---    // NOTE: if this is set when the entities original owner has already left it will be
---    // marked for deletion (similar to just calling DELETE_ENTITY)
---    DeleteOnOwnerDisconnect = 1,
---    // The entity will never be deleted by the server when it does relevancy checks
---    // you should only use this on entities that need to be relatively persistent
---    KeepEntity = 2
---}
---```
---
---Sets what the server will do when the entity no longer has its original owner. By default the server will cleanup entities that it considers "no longer relevant".
---
---When used on trains, this native will recursively call onto all attached carriages.
---
---**NOTE**: When used with `KeepEntity` (2) this native only guarantees that the ***server*** will not delete the entity, client requests to delete the entity will still work perfectly fine.
---@param entity integer
---@param orphanMode integer
function SetEntityOrphanMode(entity, orphanMode) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD3FC9D88)  
---Enables or disables the owner check for the specified entity in network-synchronized scenes. When set to `false`, the entity cannot participate in synced scenes initiated by clients that do not own the entity.
---
---By default, this is `false` for all entities, meaning only the entity's owner can include it in networked synchronized scenes.
---@param entity integer
---@param allow boolean
function SetEntityRemoteSyncedScenesAllowed(entity, allow) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA345EFE)  
---Sets the rotation of a specified entity in the game world.
---
---```
---NativeDB Introduced: v323
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_ENTITY_ROTATION](?\_0x8524A8B0171D5E07).**
---@param entity integer
---@param pitch number
---@param roll number
---@param yaw number
---@param rotationOrder integer
---@param bDeadCheck boolean
function SetEntityRotation(entity, pitch, roll, yaw, rotationOrder, bDeadCheck) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x635E5289)  
---Sets the routing bucket for the specified entity.
---
---Routing buckets are also known as 'dimensions' or 'virtual worlds' in past echoes, however they are population-aware.
---@param entity integer
---@param bucket integer
function SetEntityRoutingBucket(entity, bucket) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFF5A1988)  
---```
---Note that the third parameter(denoted as z) is "up and down" with positive numbers encouraging upwards movement.
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_ENTITY_VELOCITY](?\_0x1C99BB7B6E96D16F).**
---@param entity integer
---@param x number
---@param y number
---@param z number
function SetEntityVelocity(entity, x, y, z) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF90B7469)  
---This native does not have an official description.
---@param gametypeName string
function SetGameType(gametypeName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF5C6330C)  
---Sets the handler for HTTP requests made to the executing resource.
---
---Example request URL: `http://localhost:30120/http-test/ping` - this request will be sent to the `http-test` resource with the `/ping` path.
---
---The handler function assumes the following signature:
---
---```ts
---function HttpHandler(
---  request: {
---    address: string;
---    headers: Record<string, string>;
---    method: string;
---    path: string;
---    setDataHandler(handler: (data: string) => void): void;
---    setDataHandler(handler: (data: ArrayBuffer) => void, binary: 'binary'): void;
---    setCancelHandler(handler: () => void): void;
---  },
---  response: {
---    writeHead(code: number, headers?: Record<string, string | string[]>): void;
---    write(data: string): void;
---    send(data?: string): void;
---  }
---): void;
---```
---
---*   **request**: The request object.
---    *   **address**: The IP address of the request sender.
---    *   **path**: The path to where the request was sent.
---    *   **headers**: The headers sent with the request.
---    *   **method**: The request method.
---    *   **setDataHandler**: Sets the handler for when a data body is passed with the request. Additionally you can pass the `'binary'` argument to receive a `BufferArray` in JavaScript or `System.Byte[]` in C# (has no effect in Lua).
---    *   **setCancelHandler**: Sets the handler for when the request is cancelled.
---*   **response**: An object to control the response.
---    *   **writeHead**: Sets the status code & headers of the response. Can be only called once and won't work if called after running other response functions.
---    *   **write**: Writes to the response body without sending it. Can be called multiple times.
---    *   **send**: Writes to the response body and then sends it along with the status code & headers, finishing the request.
---@param handler function
function SetHttpHandler(handler) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB7BA82DC)  
---This native does not have an official description.
---@param mapName string
function SetMapName(mapName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xBF90DF1A)  
---```
---NativeDB Added Parameter 4: BOOL p3
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_AMMO](?\_0x14E56BC5B5DB6A19).**
---@param ped integer
---@param weaponHash integer | string
---@param ammo integer
function SetPedAmmo(ped, weaponHash, ammo) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4E3A0CC4)  
---```
---Sets the armor of the specified ped.
---ped: The Ped to set the armor of.
---amount: A value between 0 and 100 indicating the value to set the Ped's armor to.
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_ARMOUR](?\_0xCEA04D83135264CC).**
---@param ped integer
---@param amount integer
function SetPedArmour(ped, amount) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xCF1384C4)  
---SET_PED_CAN_RAGDOLL
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_CAN_RAGDOLL](?\_0xB128377056A54E2A).**
---@param ped integer
---@param toggle boolean
function SetPedCanRagdoll(ped, toggle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD4F7B05C)  
---This native is used to set component variation on a ped. Components, drawables and textures IDs are related to the ped model.
---
---### MP Freemode list of components
---
---**0**: Face
---**1**: Mask
---**2**: Hair
---**3**: Torso
---**4**: Leg
---**5**: Parachute / bag
---**6**: Shoes
---**7**: Accessory
---**8**: Undershirt
---**9**: Kevlar
---**10**: Badge
---**11**: Torso 2
---List of Component IDs
---
---```cpp
---// Components
---enum ePedVarComp
---{
---PV_COMP_INVALID = 0xFFFFFFFF,
---PV_COMP_HEAD = 0, // "HEAD"
---PV_COMP_BERD = 1, // "BEARD"
---PV_COMP_HAIR = 2, // "HAIR"
---PV_COMP_UPPR = 3, // "UPPER"
---PV_COMP_LOWR = 4, // "LOWER"
---PV_COMP_HAND = 5, // "HAND"
---PV_COMP_FEET = 6, // "FEET"
---PV_COMP_TEEF = 7, // "TEETH"
---PV_COMP_ACCS = 8, // "ACCESSORIES"
---PV_COMP_TASK = 9, // "TASK"
---PV_COMP_DECL = 10, // "DECL"
---PV_COMP_JBIB = 11, // "JBIB"
---PV_COMP_MAX = 12,
---};
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_COMPONENT_VARIATION](?\_0x262B14F48D29DE80).**
---@param ped integer
---@param componentId integer
---@param drawableId integer
---@param textureId integer
---@param paletteId integer
function SetPedComponentVariation(ped, componentId, drawableId, textureId, paletteId) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9CFBE10D)  
---```cpp
---// Potential names and hash collisions included as comments
---enum ePedConfigFlags {
---CPED_CONFIG_FLAG_CreatedByFactory = 0,
---CPED_CONFIG_FLAG_CanBeShotInVehicle = 1,
---CPED_CONFIG_FLAG_NoCriticalHits = 2,
---CPED_CONFIG_FLAG_DrownsInWater = 3,
---CPED_CONFIG_FLAG_DrownsInSinkingVehicle = 4,
---CPED_CONFIG_FLAG_DiesInstantlyWhenSwimming = 5,
---CPED_CONFIG_FLAG_HasBulletProofVest = 6,
---CPED_CONFIG_FLAG_UpperBodyDamageAnimsOnly = 7,
---CPED_CONFIG_FLAG_NeverFallOffSkis = 8,
---CPED_CONFIG_FLAG_NeverEverTargetThisPed = 9,
---CPED_CONFIG_FLAG_ThisPedIsATargetPriority = 10,
---CPED_CONFIG_FLAG_TargettableWithNoLos = 11,
---CPED_CONFIG_FLAG_DoesntListenToPlayerGroupCommands = 12,
---CPED_CONFIG_FLAG_NeverLeavesGroup = 13,
---CPED_CONFIG_FLAG_DoesntDropWeaponsWhenDead = 14,
---CPED_CONFIG_FLAG_SetDelayedWeaponAsCurrent = 15,
---CPED_CONFIG_FLAG_KeepTasksAfterCleanUp = 16,
---CPED_CONFIG_FLAG_BlockNonTemporaryEvents = 17,
---CPED_CONFIG_FLAG_HasAScriptBrain = 18,
---CPED_CONFIG_FLAG_WaitingForScriptBrainToLoad = 19,
---CPED_CONFIG_FLAG_AllowMedicsToReviveMe = 20,
---CPED_CONFIG_FLAG_MoneyHasBeenGivenByScript = 21,
---CPED_CONFIG_FLAG_NotAllowedToCrouch = 22,
---CPED_CONFIG_FLAG_DeathPickupsPersist = 23,
---CPED_CONFIG_FLAG_IgnoreSeenMelee = 24,
---CPED_CONFIG_FLAG_ForceDieIfInjured = 25,
---CPED_CONFIG_FLAG_DontDragMeOutCar = 26,
---CPED_CONFIG_FLAG_StayInCarOnJack = 27,
---CPED_CONFIG_FLAG_ForceDieInCar = 28,
---CPED_CONFIG_FLAG_GetOutUndriveableVehicle = 29,
---CPED_CONFIG_FLAG_WillRemainOnBoatAfterMissionEnds = 30,
---CPED_CONFIG_FLAG_DontStoreAsPersistent = 31,
---CPED_CONFIG_FLAG_WillFlyThroughWindscreen = 32,
---CPED_CONFIG_FLAG_DieWhenRagdoll = 33,
---CPED_CONFIG_FLAG_HasHelmet = 34,
---CPED_CONFIG_FLAG_UseHelmet = 35,
---CPED_CONFIG_FLAG_DontTakeOffHelmet = 36,
---CPED_CONFIG_FLAG_HideInCutscene = 37,
---CPED_CONFIG_FLAG_PedIsEnemyToPlayer = 38,
---CPED_CONFIG_FLAG_DisableEvasiveDives = 39,
---CPED_CONFIG_FLAG_PedGeneratesDeadBodyEvents = 40,
---CPED_CONFIG_FLAG_DontAttackPlayerWithoutWantedLevel = 41,
---CPED_CONFIG_FLAG_DontInfluenceWantedLevel = 42,
---CPED_CONFIG_FLAG_DisablePlayerLockon = 43,
---CPED_CONFIG_FLAG_DisableLockonToRandomPeds = 44,
---CPED_CONFIG_FLAG_AllowLockonToFriendlyPlayers = 45,
---_0xDB115BFA = 46,
---CPED_CONFIG_FLAG_PedBeingDeleted = 47,
---CPED_CONFIG_FLAG_BlockWeaponSwitching = 48,
---CPED_CONFIG_FLAG_BlockGroupPedAimedAtResponse = 49,
---CPED_CONFIG_FLAG_WillFollowLeaderAnyMeans = 50,
---CPED_CONFIG_FLAG_BlippedByScript = 51,
---CPED_CONFIG_FLAG_DrawRadarVisualField = 52,
---CPED_CONFIG_FLAG_StopWeaponFiringOnImpact = 53,
---CPED_CONFIG_FLAG_DissableAutoFallOffTests = 54,
---CPED_CONFIG_FLAG_SteerAroundDeadBodies = 55,
---CPED_CONFIG_FLAG_ConstrainToNavMesh = 56,
---CPED_CONFIG_FLAG_SyncingAnimatedProps = 57,
---CPED_CONFIG_FLAG_IsFiring = 58,
---CPED_CONFIG_FLAG_WasFiring = 59,
---CPED_CONFIG_FLAG_IsStanding = 60,
---CPED_CONFIG_FLAG_WasStanding = 61,
---CPED_CONFIG_FLAG_InVehicle = 62,
---CPED_CONFIG_FLAG_OnMount = 63,
---CPED_CONFIG_FLAG_AttachedToVehicle = 64,
---CPED_CONFIG_FLAG_IsSwimming = 65,
---CPED_CONFIG_FLAG_WasSwimming = 66,
---CPED_CONFIG_FLAG_IsSkiing = 67,
---CPED_CONFIG_FLAG_IsSitting = 68,
---CPED_CONFIG_FLAG_KilledByStealth = 69,
---CPED_CONFIG_FLAG_KilledByTakedown = 70,
---CPED_CONFIG_FLAG_Knockedout = 71,
---CPED_CONFIG_FLAG_ClearRadarBlipOnDeath = 72,
---CPED_CONFIG_FLAG_JustGotOffTrain = 73,
---CPED_CONFIG_FLAG_JustGotOnTrain = 74,
---CPED_CONFIG_FLAG_UsingCoverPoint = 75,
---CPED_CONFIG_FLAG_IsInTheAir = 76,
---CPED_CONFIG_FLAG_KnockedUpIntoAir = 77,
---CPED_CONFIG_FLAG_IsAimingGun = 78,
---CPED_CONFIG_FLAG_HasJustLeftCar = 79,
---CPED_CONFIG_FLAG_TargetWhenInjuredAllowed = 80,
---CPED_CONFIG_FLAG_CurrLeftFootCollNM = 81,
---CPED_CONFIG_FLAG_PrevLeftFootCollNM = 82,
---CPED_CONFIG_FLAG_CurrRightFootCollNM = 83,
---CPED_CONFIG_FLAG_PrevRightFootCollNM = 84,
---CPED_CONFIG_FLAG_HasBeenBumpedInCar = 85,
---CPED_CONFIG_FLAG_InWaterTaskQuitToClimbLadder = 86,
---CPED_CONFIG_FLAG_NMTwoHandedWeaponBothHandsConstrained = 87,
---CPED_CONFIG_FLAG_CreatedBloodPoolTimer = 88,
---CPED_CONFIG_FLAG_DontActivateRagdollFromAnyPedImpact = 89,
---CPED_CONFIG_FLAG_GroupPedFailedToEnterCover = 90,
---CPED_CONFIG_FLAG_AlreadyChattedOnPhone = 91,
---CPED_CONFIG_FLAG_AlreadyReactedToPedOnRoof = 92,
---CPED_CONFIG_FLAG_ForcePedLoadCover = 93,
---CPED_CONFIG_FLAG_BlockCoweringInCover = 94,
---CPED_CONFIG_FLAG_BlockPeekingInCover = 95,
---CPED_CONFIG_FLAG_JustLeftCarNotCheckedForDoors = 96,
---CPED_CONFIG_FLAG_VaultFromCover = 97,
---CPED_CONFIG_FLAG_AutoConversationLookAts = 98,
---CPED_CONFIG_FLAG_UsingCrouchedPedCapsule = 99,
---CPED_CONFIG_FLAG_HasDeadPedBeenReported = 100,
---CPED_CONFIG_FLAG_ForcedAim = 101,
---CPED_CONFIG_FLAG_SteersAroundPeds = 102,
---CPED_CONFIG_FLAG_SteersAroundObjects = 103,
---CPED_CONFIG_FLAG_OpenDoorArmIK = 104,
---CPED_CONFIG_FLAG_ForceReload = 105,
---CPED_CONFIG_FLAG_DontActivateRagdollFromVehicleImpact = 106,
---CPED_CONFIG_FLAG_DontActivateRagdollFromBulletImpact = 107,
---CPED_CONFIG_FLAG_DontActivateRagdollFromExplosions = 108,
---CPED_CONFIG_FLAG_DontActivateRagdollFromFire = 109,
---CPED_CONFIG_FLAG_DontActivateRagdollFromElectrocution = 110,
---CPED_CONFIG_FLAG_IsBeingDraggedToSafety = 111,
---CPED_CONFIG_FLAG_HasBeenDraggedToSafety = 112,
---CPED_CONFIG_FLAG_KeepWeaponHolsteredUnlessFired = 113,
---CPED_CONFIG_FLAG_ForceScriptControlledKnockout = 114,
---CPED_CONFIG_FLAG_FallOutOfVehicleWhenKilled = 115,
---CPED_CONFIG_FLAG_GetOutBurningVehicle = 116,
---CPED_CONFIG_FLAG_BumpedByPlayer = 117,
---CPED_CONFIG_FLAG_RunFromFiresAndExplosions = 118,
---CPED_CONFIG_FLAG_TreatAsPlayerDuringTargeting = 119,
---CPED_CONFIG_FLAG_IsHandCuffed = 120,
---CPED_CONFIG_FLAG_IsAnkleCuffed = 121,
---CPED_CONFIG_FLAG_DisableMelee = 122,
---CPED_CONFIG_FLAG_DisableUnarmedDrivebys = 123,
---CPED_CONFIG_FLAG_JustGetsPulledOutWhenElectrocuted = 124,
---CPED_CONFIG_FLAG_UNUSED_REPLACE_ME = 125,
---CPED_CONFIG_FLAG_WillNotHotwireLawEnforcementVehicle = 126,
---CPED_CONFIG_FLAG_WillCommandeerRatherThanJack = 127,
---CPED_CONFIG_FLAG_CanBeAgitated = 128,
---CPED_CONFIG_FLAG_ForcePedToFaceLeftInCover = 129,
---CPED_CONFIG_FLAG_ForcePedToFaceRightInCover = 130,
---CPED_CONFIG_FLAG_BlockPedFromTurningInCover = 131,
---CPED_CONFIG_FLAG_KeepRelationshipGroupAfterCleanUp = 132,
---CPED_CONFIG_FLAG_ForcePedToBeDragged = 133,
---CPED_CONFIG_FLAG_PreventPedFromReactingToBeingJacked = 134,
---CPED_CONFIG_FLAG_IsScuba = 135,
---CPED_CONFIG_FLAG_WillArrestRatherThanJack = 136,
---CPED_CONFIG_FLAG_RemoveDeadExtraFarAway = 137,
---CPED_CONFIG_FLAG_RidingTrain = 138,
---CPED_CONFIG_FLAG_ArrestResult = 139,
---CPED_CONFIG_FLAG_CanAttackFriendly = 140,
---CPED_CONFIG_FLAG_WillJackAnyPlayer = 141,
---CPED_CONFIG_FLAG_BumpedByPlayerVehicle = 142,
---CPED_CONFIG_FLAG_DodgedPlayerVehicle = 143,
---CPED_CONFIG_FLAG_WillJackWantedPlayersRatherThanStealCar = 144,
---CPED_CONFIG_FLAG_NoCopWantedAggro = 145,
---CPED_CONFIG_FLAG_DisableLadderClimbing = 146,
---CPED_CONFIG_FLAG_StairsDetected = 147,
---CPED_CONFIG_FLAG_SlopeDetected = 148,
---CPED_CONFIG_FLAG_HelmetHasBeenShot = 149,
---CPED_CONFIG_FLAG_CowerInsteadOfFlee = 150,
---CPED_CONFIG_FLAG_CanActivateRagdollWhenVehicleUpsideDown = 151,
---CPED_CONFIG_FLAG_AlwaysRespondToCriesForHelp = 152,
---CPED_CONFIG_FLAG_DisableBloodPoolCreation = 153,
---CPED_CONFIG_FLAG_ShouldFixIfNoCollision = 154,
---CPED_CONFIG_FLAG_CanPerformArrest = 155,
---CPED_CONFIG_FLAG_CanPerformUncuff = 156,
---CPED_CONFIG_FLAG_CanBeArrested = 157,
---CPED_CONFIG_FLAG_MoverConstrictedByOpposingCollisions = 158,
---CPED_CONFIG_FLAG_PlayerPreferFrontSeatMP = 159,
---CPED_CONFIG_FLAG_DontActivateRagdollFromImpactObject = 160,
---CPED_CONFIG_FLAG_DontActivateRagdollFromMelee = 161,
---CPED_CONFIG_FLAG_DontActivateRagdollFromWaterJet = 162,
---CPED_CONFIG_FLAG_DontActivateRagdollFromDrowning = 163,
---CPED_CONFIG_FLAG_DontActivateRagdollFromFalling = 164,
---CPED_CONFIG_FLAG_DontActivateRagdollFromRubberBullet = 165,
---CPED_CONFIG_FLAG_IsInjured = 166,
---CPED_CONFIG_FLAG_DontEnterVehiclesInPlayersGroup = 167,
---CPED_CONFIG_FLAG_SwimmingTasksRunning = 168,
---CPED_CONFIG_FLAG_PreventAllMeleeTaunts = 169,
---CPED_CONFIG_FLAG_ForceDirectEntry = 170,
---CPED_CONFIG_FLAG_AlwaysSeeApproachingVehicles = 171,
---CPED_CONFIG_FLAG_CanDiveAwayFromApproachingVehicles = 172,
---CPED_CONFIG_FLAG_AllowPlayerToInterruptVehicleEntryExit = 173,
---CPED_CONFIG_FLAG_OnlyAttackLawIfPlayerIsWanted = 174,
---CPED_CONFIG_FLAG_PlayerInContactWithKinematicPed = 175,
---CPED_CONFIG_FLAG_PlayerInContactWithSomethingOtherThanKinematicPed = 176,
---CPED_CONFIG_FLAG_PedsJackingMeDontGetIn = 177,
---CPED_CONFIG_FLAG_AdditionalRappellingPed = 178,
---CPED_CONFIG_FLAG_PedIgnoresAnimInterruptEvents = 179,
---CPED_CONFIG_FLAG_IsInCustody = 180,
---CPED_CONFIG_FLAG_ForceStandardBumpReactionThresholds = 181,
---CPED_CONFIG_FLAG_LawWillOnlyAttackIfPlayerIsWanted = 182,
---CPED_CONFIG_FLAG_IsAgitated = 183,
---CPED_CONFIG_FLAG_PreventAutoShuffleToDriversSeat = 184,
---CPED_CONFIG_FLAG_UseKinematicModeWhenStationary = 185,
---CPED_CONFIG_FLAG_EnableWeaponBlocking = 186,
---CPED_CONFIG_FLAG_HasHurtStarted = 187,
---CPED_CONFIG_FLAG_DisableHurt = 188,
---CPED_CONFIG_FLAG_PlayerIsWeird = 189,
---CPED_CONFIG_FLAG_PedHadPhoneConversation = 190,
---CPED_CONFIG_FLAG_BeganCrossingRoad = 191,
---CPED_CONFIG_FLAG_WarpIntoLeadersVehicle = 192,
---CPED_CONFIG_FLAG_DoNothingWhenOnFootByDefault = 193,
---CPED_CONFIG_FLAG_UsingScenario = 194,
---CPED_CONFIG_FLAG_VisibleOnScreen = 195,
---CPED_CONFIG_FLAG_DontCollideWithKinematic = 196,
---CPED_CONFIG_FLAG_ActivateOnSwitchFromLowPhysicsLod = 197,
---CPED_CONFIG_FLAG_DontActivateRagdollOnPedCollisionWhenDead = 198,
---CPED_CONFIG_FLAG_DontActivateRagdollOnVehicleCollisionWhenDead = 199,
---CPED_CONFIG_FLAG_HasBeenInArmedCombat = 200,
---CPED_CONFIG_FLAG_UseDiminishingAmmoRate = 201,
---CPED_CONFIG_FLAG_Avoidance_Ignore_All = 202,
---CPED_CONFIG_FLAG_Avoidance_Ignored_by_All = 203,
---CPED_CONFIG_FLAG_Avoidance_Ignore_Group1 = 204,
---CPED_CONFIG_FLAG_Avoidance_Member_of_Group1 = 205,
---CPED_CONFIG_FLAG_ForcedToUseSpecificGroupSeatIndex = 206,
---CPED_CONFIG_FLAG_LowPhysicsLodMayPlaceOnNavMesh = 207,
---CPED_CONFIG_FLAG_DisableExplosionReactions = 208,
---CPED_CONFIG_FLAG_DodgedPlayer = 209,
---CPED_CONFIG_FLAG_WaitingForPlayerControlInterrupt = 210,
---CPED_CONFIG_FLAG_ForcedToStayInCover = 211,
---CPED_CONFIG_FLAG_GeneratesSoundEvents = 212,
---CPED_CONFIG_FLAG_ListensToSoundEvents = 213,
---CPED_CONFIG_FLAG_AllowToBeTargetedInAVehicle = 214,
---CPED_CONFIG_FLAG_WaitForDirectEntryPointToBeFreeWhenExiting = 215,
---CPED_CONFIG_FLAG_OnlyRequireOnePressToExitVehicle = 216,
---CPED_CONFIG_FLAG_ForceExitToSkyDive = 217,
---CPED_CONFIG_FLAG_SteersAroundVehicles = 218,
---CPED_CONFIG_FLAG_AllowPedInVehiclesOverrideTaskFlags = 219,
---CPED_CONFIG_FLAG_DontEnterLeadersVehicle = 220,
---CPED_CONFIG_FLAG_DisableExitToSkyDive = 221,
---CPED_CONFIG_FLAG_ScriptHasDisabledCollision = 222,
---CPED_CONFIG_FLAG_UseAmbientModelScaling = 223,
---CPED_CONFIG_FLAG_DontWatchFirstOnNextHurryAway = 224,
---CPED_CONFIG_FLAG_DisablePotentialToBeWalkedIntoResponse = 225,
---CPED_CONFIG_FLAG_DisablePedAvoidance = 226,
---CPED_CONFIG_FLAG_ForceRagdollUponDeath = 227,
---CPED_CONFIG_FLAG_CanLosePropsOnDamage = 228,
---CPED_CONFIG_FLAG_DisablePanicInVehicle = 229,
---CPED_CONFIG_FLAG_AllowedToDetachTrailer = 230,
---CPED_CONFIG_FLAG_HasShotBeenReactedToFromFront = 231,
---CPED_CONFIG_FLAG_HasShotBeenReactedToFromBack = 232,
---CPED_CONFIG_FLAG_HasShotBeenReactedToFromLeft = 233,
---CPED_CONFIG_FLAG_HasShotBeenReactedToFromRight = 234,
---CPED_CONFIG_FLAG_AllowBlockDeadPedRagdollActivation = 235,
---CPED_CONFIG_FLAG_IsHoldingProp = 236,
---CPED_CONFIG_FLAG_BlocksPathingWhenDead = 237,
---CPED_CONFIG_FLAG_ForcePlayNormalScenarioExitOnNextScriptCommand = 238,
---CPED_CONFIG_FLAG_ForcePlayImmediateScenarioExitOnNextScriptCommand = 239,
---CPED_CONFIG_FLAG_ForceSkinCharacterCloth = 240,
---CPED_CONFIG_FLAG_LeaveEngineOnWhenExitingVehicles = 241,
---CPED_CONFIG_FLAG_PhoneDisableTextingAnimations = 242,
---CPED_CONFIG_FLAG_PhoneDisableTalkingAnimations = 243,
---CPED_CONFIG_FLAG_PhoneDisableCameraAnimations = 244,
---CPED_CONFIG_FLAG_DisableBlindFiringInShotReactions = 245,
---CPED_CONFIG_FLAG_AllowNearbyCoverUsage = 246,
---CPED_CONFIG_FLAG_InStrafeTransition = 247,
---CPED_CONFIG_FLAG_CanPlayInCarIdles = 248,
---CPED_CONFIG_FLAG_CanAttackNonWantedPlayerAsLaw = 249,
---CPED_CONFIG_FLAG_WillTakeDamageWhenVehicleCrashes = 250,
---CPED_CONFIG_FLAG_AICanDrivePlayerAsRearPassenger = 251,
---CPED_CONFIG_FLAG_PlayerCanJackFriendlyPlayers = 252,
---CPED_CONFIG_FLAG_OnStairs = 253,
---CPED_CONFIG_FLAG_SimulatingAiming = 254,
---CPED_CONFIG_FLAG_AIDriverAllowFriendlyPassengerSeatEntry = 255,
---CPED_CONFIG_FLAG_ParentCarIsBeingRemoved = 256,
---CPED_CONFIG_FLAG_AllowMissionPedToUseInjuredMovement = 257,
---CPED_CONFIG_FLAG_CanLoseHelmetOnDamage = 258,
---CPED_CONFIG_FLAG_NeverDoScenarioExitProbeChecks = 259,
---CPED_CONFIG_FLAG_SuppressLowLODRagdollSwitchWhenCorpseSettles = 260,
---CPED_CONFIG_FLAG_PreventUsingLowerPrioritySeats = 261,
---CPED_CONFIG_FLAG_JustLeftVehicleNeedsReset = 262,
---CPED_CONFIG_FLAG_TeleportIfCantReachPlayer = 263,
---CPED_CONFIG_FLAG_PedsInVehiclePositionNeedsReset = 264,
---CPED_CONFIG_FLAG_PedsFullyInSeat = 265,
---CPED_CONFIG_FLAG_AllowPlayerLockOnIfFriendly = 266,
---CPED_CONFIG_FLAG_UseCameraHeadingForDesiredDirectionLockOnTest = 267,
---CPED_CONFIG_FLAG_TeleportToLeaderVehicle = 268,
---CPED_CONFIG_FLAG_Avoidance_Ignore_WeirdPedBuffer = 269,
---CPED_CONFIG_FLAG_OnStairSlope = 270,
---CPED_CONFIG_FLAG_HasPlayedNMGetup = 271,
---CPED_CONFIG_FLAG_DontBlipCop = 272,
---CPED_CONFIG_FLAG_SpawnedAtExtendedRangeScenario = 273,
---CPED_CONFIG_FLAG_WalkAlongsideLeaderWhenClose = 274,
---CPED_CONFIG_FLAG_KillWhenTrapped = 275,
---CPED_CONFIG_FLAG_EdgeDetected = 276,
---CPED_CONFIG_FLAG_AlwaysWakeUpPhysicsOfIntersectedPeds = 277,
---CPED_CONFIG_FLAG_EquippedAmbientLoadOutWeapon = 278,
---CPED_CONFIG_FLAG_AvoidTearGas = 279,
---CPED_CONFIG_FLAG_StoppedSpeechUponFreezing = 280,
---CPED_CONFIG_FLAG_DisableGoToWritheWhenInjured = 281,
---CPED_CONFIG_FLAG_OnlyUseForcedSeatWhenEnteringHeliInGroup = 282,
---CPED_CONFIG_FLAG_ThrownFromVehicleDueToExhaustion = 283,
---CPED_CONFIG_FLAG_UpdateEnclosedSearchRegion = 284,
---CPED_CONFIG_FLAG_DisableWeirdPedEvents = 285,
---CPED_CONFIG_FLAG_ShouldChargeNow = 286,
---CPED_CONFIG_FLAG_RagdollingOnBoat = 287,
---CPED_CONFIG_FLAG_HasBrandishedWeapon = 288,
---CPED_CONFIG_FLAG_AllowMinorReactionsAsMissionPed = 289,
---CPED_CONFIG_FLAG_BlockDeadBodyShockingEventsWhenDead = 290,
---CPED_CONFIG_FLAG_PedHasBeenSeen = 291,
---CPED_CONFIG_FLAG_PedIsInReusePool = 292,
---CPED_CONFIG_FLAG_PedWasReused = 293,
---CPED_CONFIG_FLAG_DisableShockingEvents = 294,
---CPED_CONFIG_FLAG_MovedUsingLowLodPhysicsSinceLastActive = 295,
---CPED_CONFIG_FLAG_NeverReactToPedOnRoof = 296,
---CPED_CONFIG_FLAG_ForcePlayFleeScenarioExitOnNextScriptCommand = 297,
---CPED_CONFIG_FLAG_JustBumpedIntoVehicle = 298,
---CPED_CONFIG_FLAG_DisableShockingDrivingOnPavementEvents = 299,
---CPED_CONFIG_FLAG_ShouldThrowSmokeNow = 300,
---CPED_CONFIG_FLAG_DisablePedConstraints = 301,
---CPED_CONFIG_FLAG_ForceInitialPeekInCover = 302,
---CPED_CONFIG_FLAG_CreatedByDispatch = 303,
---CPED_CONFIG_FLAG_PointGunLeftHandSupporting = 304,
---CPED_CONFIG_FLAG_DisableJumpingFromVehiclesAfterLeader = 305,
---CPED_CONFIG_FLAG_DontActivateRagdollFromPlayerPedImpact = 306,
---CPED_CONFIG_FLAG_DontActivateRagdollFromAiRagdollImpact = 307,
---CPED_CONFIG_FLAG_DontActivateRagdollFromPlayerRagdollImpact = 308,
---CPED_CONFIG_FLAG_DisableQuadrupedSpring = 309,
---CPED_CONFIG_FLAG_IsInCluster = 310,
---CPED_CONFIG_FLAG_ShoutToGroupOnPlayerMelee = 311,
---CPED_CONFIG_FLAG_IgnoredByAutoOpenDoors = 312,
---CPED_CONFIG_FLAG_PreferInjuredGetup = 313,
---CPED_CONFIG_FLAG_ForceIgnoreMeleeActiveCombatant = 314,
---CPED_CONFIG_FLAG_CheckLoSForSoundEvents = 315,
---CPED_CONFIG_FLAG_JackedAbandonedCar = 316,
---CPED_CONFIG_FLAG_CanSayFollowedByPlayerAudio = 317,
---CPED_CONFIG_FLAG_ActivateRagdollFromMinorPlayerContact = 318,
---CPED_CONFIG_FLAG_HasPortablePickupAttached = 319,
---CPED_CONFIG_FLAG_ForcePoseCharacterCloth = 320,
---CPED_CONFIG_FLAG_HasClothCollisionBounds = 321,
---CPED_CONFIG_FLAG_HasHighHeels = 322,
---CPED_CONFIG_FLAG_TreatAsAmbientPedForDriverLockOn = 323,
---CPED_CONFIG_FLAG_DontBehaveLikeLaw = 324,
---CPED_CONFIG_FLAG_SpawnedAtScenario = 325,
---CPED_CONFIG_FLAG_DisablePoliceInvestigatingBody = 326,
---CPED_CONFIG_FLAG_DisableWritheShootFromGround = 327,
---CPED_CONFIG_FLAG_LowerPriorityOfWarpSeats = 328,
---CPED_CONFIG_FLAG_DisableTalkTo = 329,
---CPED_CONFIG_FLAG_DontBlip = 330,
---CPED_CONFIG_FLAG_IsSwitchingWeapon = 331,
---CPED_CONFIG_FLAG_IgnoreLegIkRestrictions = 332,
---CPED_CONFIG_FLAG_ScriptForceNoTimesliceIntelligenceUpdate = 333,
---CPED_CONFIG_FLAG_JackedOutOfMyVehicle = 334,
---CPED_CONFIG_FLAG_WentIntoCombatAfterBeingJacked = 335,
---CPED_CONFIG_FLAG_DontActivateRagdollForVehicleGrab = 336,
---CPED_CONFIG_FLAG_ForcePackageCharacterCloth = 337,
---CPED_CONFIG_FLAG_DontRemoveWithValidOrder = 338,
---CPED_CONFIG_FLAG_AllowTaskDoNothingTimeslicing = 339,
---CPED_CONFIG_FLAG_ForcedToStayInCoverDueToPlayerSwitch = 340,
---CPED_CONFIG_FLAG_ForceProneCharacterCloth = 341,
---CPED_CONFIG_FLAG_NotAllowedToJackAnyPlayers = 342,
---CPED_CONFIG_FLAG_InToStrafeTransition = 343,
---CPED_CONFIG_FLAG_KilledByStandardMelee = 344,
---CPED_CONFIG_FLAG_AlwaysLeaveTrainUponArrival = 345,
---CPED_CONFIG_FLAG_ForcePlayDirectedNormalScenarioExitOnNextScriptCommand = 346,
---CPED_CONFIG_FLAG_OnlyWritheFromWeaponDamage = 347,
---CPED_CONFIG_FLAG_UseSloMoBloodVfx = 348,
---CPED_CONFIG_FLAG_EquipJetpack = 349,
---CPED_CONFIG_FLAG_PreventDraggedOutOfCarThreatResponse = 350,
---CPED_CONFIG_FLAG_ScriptHasCompletelyDisabledCollision = 351,
---CPED_CONFIG_FLAG_NeverDoScenarioNavChecks = 352,
---CPED_CONFIG_FLAG_ForceSynchronousScenarioExitChecking = 353,
---CPED_CONFIG_FLAG_ThrowingGrenadeWhileAiming = 354,
---CPED_CONFIG_FLAG_HeadbobToRadioEnabled = 355,
---CPED_CONFIG_FLAG_ForceDeepSurfaceCheck = 356,
---CPED_CONFIG_FLAG_DisableDeepSurfaceAnims = 357,
---CPED_CONFIG_FLAG_DontBlipNotSynced = 358,
---CPED_CONFIG_FLAG_IsDuckingInVehicle = 359,
---CPED_CONFIG_FLAG_PreventAutoShuffleToTurretSeat = 360,
---CPED_CONFIG_FLAG_DisableEventInteriorStatusCheck = 361,
---CPED_CONFIG_FLAG_HasReserveParachute = 362,
---CPED_CONFIG_FLAG_UseReserveParachute = 363,
---CPED_CONFIG_FLAG_TreatDislikeAsHateWhenInCombat = 364,
---CPED_CONFIG_FLAG_OnlyUpdateTargetWantedIfSeen = 365,
---CPED_CONFIG_FLAG_AllowAutoShuffleToDriversSeat = 366,
---CPED_CONFIG_FLAG_DontActivateRagdollFromSmokeGrenade = 367,
---CPED_CONFIG_FLAG_LinkMBRToOwnerOnChain = 368,
---CPED_CONFIG_FLAG_AmbientFriendBumpedByPlayer = 369,
---CPED_CONFIG_FLAG_AmbientFriendBumpedByPlayerVehicle = 370,
---CPED_CONFIG_FLAG_InFPSUnholsterTransition = 371,
---CPED_CONFIG_FLAG_PreventReactingToSilencedCloneBullets = 372,
---CPED_CONFIG_FLAG_DisableInjuredCryForHelpEvents = 373,
---CPED_CONFIG_FLAG_NeverLeaveTrain = 374,
---CPED_CONFIG_FLAG_DontDropJetpackOnDeath = 375,
---CPED_CONFIG_FLAG_UseFPSUnholsterTransitionDuringCombatRoll = 376,
---CPED_CONFIG_FLAG_ExitingFPSCombatRoll = 377,
---CPED_CONFIG_FLAG_ScriptHasControlOfPlayer = 378,
---CPED_CONFIG_FLAG_PlayFPSIdleFidgetsForProjectile = 379,
---CPED_CONFIG_FLAG_DisableAutoEquipHelmetsInBikes = 380,
---CPED_CONFIG_FLAG_DisableAutoEquipHelmetsInAircraft = 381,
---CPED_CONFIG_FLAG_WasPlayingFPSGetup = 382,
---CPED_CONFIG_FLAG_WasPlayingFPSMeleeActionResult = 383,
---CPED_CONFIG_FLAG_PreferNoPriorityRemoval = 384,
---CPED_CONFIG_FLAG_FPSFidgetsAbortedOnFire = 385,
---CPED_CONFIG_FLAG_ForceFPSIKWithUpperBodyAnim = 386,
---CPED_CONFIG_FLAG_SwitchingCharactersInFirstPerson = 387,
---CPED_CONFIG_FLAG_IsClimbingLadder = 388,
---CPED_CONFIG_FLAG_HasBareFeet = 389,
---CPED_CONFIG_FLAG_UNUSED_REPLACE_ME_2 = 390,
---CPED_CONFIG_FLAG_GoOnWithoutVehicleIfItIsUnableToGetBackToRoad = 391,
---CPED_CONFIG_FLAG_BlockDroppingHealthSnacksOnDeath = 392,
---CPED_CONFIG_FLAG_ResetLastVehicleOnVehicleExit = 393,
---CPED_CONFIG_FLAG_ForceThreatResponseToNonFriendToFriendMeleeActions = 394,
---CPED_CONFIG_FLAG_DontRespondToRandomPedsDamage = 395,
---CPED_CONFIG_FLAG_AllowContinuousThreatResponseWantedLevelUpdates = 396,
---CPED_CONFIG_FLAG_KeepTargetLossResponseOnCleanup = 397,
---CPED_CONFIG_FLAG_PlayersDontDragMeOutOfCar = 398,
---CPED_CONFIG_FLAG_BroadcastRepondedToThreatWhenGoingToPointShooting = 399,
---CPED_CONFIG_FLAG_IgnorePedTypeForIsFriendlyWith = 400,
---CPED_CONFIG_FLAG_TreatNonFriendlyAsHateWhenInCombat = 401,
---CPED_CONFIG_FLAG_DontLeaveVehicleIfLeaderNotInVehicle = 402,
---CPED_CONFIG_FLAG_ChangeFromPermanentToAmbientPopTypeOnMigration = 403,
---CPED_CONFIG_FLAG_AllowMeleeReactionIfMeleeProofIsOn = 404,
---CPED_CONFIG_FLAG_UsingLowriderLeans = 405,
---CPED_CONFIG_FLAG_UsingAlternateLowriderLeans = 406,
---CPED_CONFIG_FLAG_UseNormalExplosionDamageWhenBlownUpInVehicle = 407,
---CPED_CONFIG_FLAG_DisableHomingMissileLockForVehiclePedInside = 408,
---CPED_CONFIG_FLAG_DisableTakeOffScubaGear = 409,
---CPED_CONFIG_FLAG_IgnoreMeleeFistWeaponDamageMult = 410,
---CPED_CONFIG_FLAG_LawPedsCanFleeFromNonWantedPlayer = 411,
---CPED_CONFIG_FLAG_ForceBlipSecurityPedsIfPlayerIsWanted = 412,
---CPED_CONFIG_FLAG_IsHolsteringWeapon = 413,
---CPED_CONFIG_FLAG_UseGoToPointForScenarioNavigation = 414,
---CPED_CONFIG_FLAG_DontClearLocalPassengersWantedLevel = 415,
---CPED_CONFIG_FLAG_BlockAutoSwapOnWeaponPickups = 416,
---CPED_CONFIG_FLAG_ThisPedIsATargetPriorityForAI = 417,
---CPED_CONFIG_FLAG_IsSwitchingHelmetVisor = 418,
---CPED_CONFIG_FLAG_ForceHelmetVisorSwitch = 419,
---CPED_CONFIG_FLAG_IsPerformingVehicleMelee = 420,
---CPED_CONFIG_FLAG_UseOverrideFootstepPtFx = 421,
---CPED_CONFIG_FLAG_DisableVehicleCombat = 422,
---CPED_CONFIG_FLAG_TreatAsFriendlyForTargetingAndDamage = 423,
---CPED_CONFIG_FLAG_AllowBikeAlternateAnimations = 424,
---CPED_CONFIG_FLAG_TreatAsFriendlyForTargetingAndDamageNonSynced = 425,
---CPED_CONFIG_FLAG_UseLockpickVehicleEntryAnimations = 426,
---CPED_CONFIG_FLAG_IgnoreInteriorCheckForSprinting = 427,
---CPED_CONFIG_FLAG_SwatHeliSpawnWithinLastSpottedLocation = 428,
---CPED_CONFIG_FLAG_DisableStartEngine = 429,
---CPED_CONFIG_FLAG_IgnoreBeingOnFire = 430,
---CPED_CONFIG_FLAG_DisableTurretOrRearSeatPreference = 431,
---CPED_CONFIG_FLAG_DisableWantedHelicopterSpawning = 432,
---CPED_CONFIG_FLAG_UseTargetPerceptionForCreatingAimedAtEvents = 433,
---CPED_CONFIG_FLAG_DisableHomingMissileLockon = 434,
---CPED_CONFIG_FLAG_ForceIgnoreMaxMeleeActiveSupportCombatants = 435,
---CPED_CONFIG_FLAG_StayInDefensiveAreaWhenInVehicle = 436,
---CPED_CONFIG_FLAG_DontShoutTargetPosition = 437,
---CPED_CONFIG_FLAG_DisableHelmetArmor = 438,
---CPED_CONFIG_FLAG_CreatedByConcealedPlayer = 439,
---CPED_CONFIG_FLAG_PermanentlyDisablePotentialToBeWalkedIntoResponse = 440,
---CPED_CONFIG_FLAG_PreventVehExitDueToInvalidWeapon = 441,
---CPED_CONFIG_FLAG_IgnoreNetSessionFriendlyFireCheckForAllowDamage = 442,
---CPED_CONFIG_FLAG_DontLeaveCombatIfTargetPlayerIsAttackedByPolice = 443,
---CPED_CONFIG_FLAG_CheckLockedBeforeWarp = 444,
---CPED_CONFIG_FLAG_DontShuffleInVehicleToMakeRoom = 445,
---CPED_CONFIG_FLAG_GiveWeaponOnGetup = 446,
---CPED_CONFIG_FLAG_DontHitVehicleWithProjectiles = 447,
---CPED_CONFIG_FLAG_DisableForcedEntryForOpenVehiclesFromTryLockedDoor = 448,
---CPED_CONFIG_FLAG_FiresDummyRockets = 449,
---CPED_CONFIG_FLAG_PedIsArresting = 450,
---CPED_CONFIG_FLAG_IsDecoyPed = 451,
---CPED_CONFIG_FLAG_HasEstablishedDecoy = 452,
---CPED_CONFIG_FLAG_BlockDispatchedHelicoptersFromLanding = 453,
---CPED_CONFIG_FLAG_DontCryForHelpOnStun = 454,
---CPED_CONFIG_FLAG_HitByTranqWeapon = 455,
---CPED_CONFIG_FLAG_CanBeIncapacitated = 456,
---CPED_CONFIG_FLAG_ForcedAimFromArrest = 457,
---CPED_CONFIG_FLAG_DontChangeTargetFromMelee = 458,
---_0x4376ABF2 = 459,
---CPED_CONFIG_FLAG_RagdollFloatsIndefinitely = 460,
---CPED_CONFIG_FLAG_BlockElectricWeaponDamage = 461,
---_0x262A3B8E = 462,
---_0x1AA79A25 = 463,
---}
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_CONFIG_FLAG](?\_0x1913FE4CBF41C463).**
---@param ped integer
---@param flagId integer
---@param value boolean
function SetPedConfigFlag(ped, flagId, value) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC866A984)  
---```
---Sets Ped Default Clothes
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_DEFAULT_COMPONENT_VARIATION](?\_0x45EEE61580806D63).**
---@param ped integer
function SetPedDefaultComponentVariation(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA23FE32C)  
---Sets the tint index for the hair on the specified ped.
---
---```
---NativeDB Introduced: v323
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_HAIR_TINT](?\_0x4CFFC65454C93A49).**
---@param ped integer
---@param colorID integer
---@param highlightColorID integer
function SetPedHairTint(ped, colorID, highlightColorID) end

---@deprecated
_setPedHairColor = SetPedHairTint

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x60746B88)  
---For more info please refer to [this](https://gtaforums.com/topic/858970-all-gtao-face-ids-pedset-ped-head-blend-data-explained) topic.
---**Other information:**
---IDs start at zero and go Male Non-DLC, Female Non-DLC, Male DLC, and Female DLC.</br>
---This native function is often called prior to calling natives such as:
---
---*   [`SetPedHairColor`](#\_0xA23FE32C)
---*   [`SetPedHeadOverlayColor`](#\_0x78935A27)
---*   [`SetPedHeadOverlay`](#\_0xD28DBA90)
---*   [`SetPedFaceFeature`](#\_0x6C8D4458)
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_HEAD_BLEND_DATA](?\_0x9414E18B9434C2FE).**
---@param ped integer
---@param shapeFirstID integer
---@param shapeSecondID integer
---@param shapeThirdID integer
---@param skinFirstID integer
---@param skinSecondID integer
---@param skinThirdID integer
---@param shapeMix number
---@param skinMix number
---@param thirdMix number
---@param isParent boolean
function SetPedHeadBlendData(ped, shapeFirstID, shapeSecondID, shapeThirdID, skinFirstID, skinSecondID, skinThirdID, shapeMix, skinMix, thirdMix, isParent) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD28DBA90)  
---```
---OverlayID ranges from 0 to 12, index from 0 to _GET_NUM_OVERLAY_VALUES(overlayID)-1, and opacity from 0.0 to 1.0.
---overlayID       Part                  Index, to disable
---0               Blemishes             0 - 23, 255
---1               Facial Hair           0 - 28, 255
---2               Eyebrows              0 - 33, 255
---3               Ageing                0 - 14, 255
---4               Makeup                0 - 74, 255
---5               Blush                 0 - 6, 255
---6               Complexion            0 - 11, 255
---7               Sun Damage            0 - 10, 255
---8               Lipstick              0 - 9, 255
---9               Moles/Freckles        0 - 17, 255
---10              Chest Hair            0 - 16, 255
---11              Body Blemishes        0 - 11, 255
---12              Add Body Blemishes    0 - 1, 255
---```
---
---**Note:**
---You may need to call [`SetPedHeadBlendData`](#\_0x9414E18B9434C2FE) prior to calling this native in order for it to work.
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_HEAD_OVERLAY](?\_0x48F44967FA05CC1E).**
---@param ped integer
---@param overlayID integer
---@param index integer
---@param opacity number
function SetPedHeadOverlay(ped, overlayID, index, opacity) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7500C79)  
---SET_PED_INTO_VEHICLE
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_INTO_VEHICLE](?\_0xF75B0D629E1C063D).**
---@param ped integer
---@param vehicle integer
---@param seatIndex integer
function SetPedIntoVehicle(ped, vehicle, seatIndex) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x829F2E2)  
---This native is used to set prop variation on a ped. Components, drawables and textures IDs are related to the ped model.
---
---### MP Freemode list of props
---
---**0**: Hats
---**1**: Glasses
---**2**: Ears
---**6**: Watches
---**7**: Bracelets
---List of Prop IDs
---
---```cpp
---enum eAnchorPoints
---{
---ANCHOR_HEAD = 0, // "p_head"
---ANCHOR_EYES = 1, // "p_eyes"
---ANCHOR_EARS = 2, // "p_ears"
---ANCHOR_MOUTH = 3, // "p_mouth"
---ANCHOR_LEFT_HAND = 4, // "p_lhand"
---ANCHOR_RIGHT_HAND = 5, // "p_rhand"
---ANCHOR_LEFT_WRIST = 6, // "p_lwrist"
---ANCHOR_RIGHT_WRIST = 7, // "p_rwrist"
---ANCHOR_HIP = 8, // "p_lhip"
---ANCHOR_LEFT_FOOT = 9, // "p_lfoot"
---ANCHOR_RIGHT_FOOT = 10, // "p_rfoot"
---ANCHOR_PH_L_HAND = 11, // "ph_lhand"
---ANCHOR_PH_R_HAND = 12, // "ph_rhand"
---NUM_ANCHORS = 13,
---};
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_PROP_INDEX](?\_0x93376B65A266EB5F).**
---@param ped integer
---@param componentId integer
---@param drawableId integer
---@param textureId integer
---@param attach boolean
function SetPedPropIndex(ped, componentId, drawableId, textureId, attach) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4111BA46)  
---```
---p1 is always 0 in R* scripts; and a quick disassembly seems to indicate that p1 is unused.
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_RANDOM_COMPONENT_VARIATION](?\_0xC8A9481A01E63C28).**
---@param ped integer
---@param p1 integer
function SetPedRandomComponentVariation(ped, p1) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xE3318E0E)  
---SET_PED_RANDOM_PROPS
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_RANDOM_PROPS](?\_0xC44AA05345C992C6).**
---@param ped integer
function SetPedRandomProps(ped) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xCFF6FF66)  
---PED::SET_PED_RESET_FLAG(PLAYER::PLAYER_PED_ID(), 240, 1);
---Known values:
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_RESET_FLAG](?\_0xC1E8A365BF3B29F2).**
---@param ped integer
---@param flagId integer
---@param doReset boolean
function SetPedResetFlag(ped, flagId, doReset) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x83CB5052)  
---p4/p5: Unusued in TU27
---
---### Ragdoll Types
---
---**0**: CTaskNMRelax
---**1**: CTaskNMScriptControl: Hardcoded not to work in networked environments.
---**Else**: CTaskNMBalance
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_TO_RAGDOLL](?\_0xAE99FB955581844A).**
---@param ped integer
---@param minTime integer
---@param maxTime integer
---@param ragdollType integer
---@param bAbortIfInjured boolean
---@param bAbortIfDead boolean
---@param bForceScriptControl boolean
function SetPedToRagdoll(ped, minTime, maxTime, ragdollType, bAbortIfInjured, bAbortIfDead, bForceScriptControl) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xFA12E286)  
---```cpp
---enum eNMFallType {
---TYPE_FROM_HIGH = 0,
---TYPE_OVER_WALL = 1,
---TYPE_DOWN_STAIRS = 2,
---TYPE_DIE_TYPES = 3,
---TYPE_DIE_FROM_HIGH = 4,
---TYPE_DIE_OVER_WALL = 5,
---TYPE_DIE_DOWN_STAIRS = 6
---}
---```
---
---```
---Return variable is never used in R*'s scripts.
---Not sure what p2 does. It seems like it would be a time judging by it's usage in R*'s scripts, but didn't seem to affect anything in my testings.
---x, y, and z are coordinates, most likely to where the ped will fall.
---p7 is probably the force of the fall, but untested, so I left the variable name the same.
---p8 to p13 are always 0f in R*'s scripts.
---(Simplified) Example of the usage of the function from R*'s scripts:
---ped::set_ped_to_ragdoll_with_fall(ped, 1500, 2000, 1, -entity::get_entity_forward_vector(ped), 1f, 0f, 0f, 0f, 0f, 0f, 0f);
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PED_TO_RAGDOLL_WITH_FALL](?\_0xD76632D99E4966C8).**
---@param ped integer
---@param minTime integer
---@param maxTime integer
---@param nFallType integer
---@param dirX number
---@param dirY number
---@param dirZ number
---@param fGroundHeight number
---@param grab1X number
---@param grab1Y number
---@param grab1Z number
---@param grab2X number
---@param grab2Y number
---@param grab2Z number
function SetPedToRagdollWithFall(ped, minTime, maxTime, nFallType, dirX, dirY, dirZ, fGroundHeight, grab1X, grab1Y, grab1Z, grab2X, grab2Y, grab2Z) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xD17AFCD8)  
---```
---Flags:
---SPC_AMBIENT_SCRIPT = (1 << 1),
---SPC_CLEAR_TASKS = (1 << 2),
---SPC_REMOVE_FIRES = (1 << 3),
---SPC_REMOVE_EXPLOSIONS = (1 << 4),
---SPC_REMOVE_PROJECTILES = (1 << 5),
---SPC_DEACTIVATE_GADGETS = (1 << 6),
---SPC_REENABLE_CONTROL_ON_DEATH = (1 << 7),
---SPC_LEAVE_CAMERA_CONTROL_ON = (1 << 8),
---SPC_ALLOW_PLAYER_DAMAGE = (1 << 9),
---SPC_DONT_STOP_OTHER_CARS_AROUND_PLAYER = (1 << 10),
---SPC_PREVENT_EVERYBODY_BACKOFF = (1 << 11),
---SPC_ALLOW_PAD_SHAKE = (1 << 12)
---See: https://alloc8or.re/gta5/doc/enums/eSetPlayerControlFlag.txt
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_PLAYER_CONTROL](?\_0x8D32347D6D4C40A2).**
---@param player integer
---@param bHasControl boolean
---@param flags integer
function SetPlayerControl(player, bHasControl, flags) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8A2FBAD4)  
---Sets the culling radius for the specified player.
---Set to `0.0` to reset.
---
---**WARNING**: Culling natives are deprecated and have known, [unfixable issues](https://forum.cfx.re/t/issue-with-culling-radius-and-server-side-entities/4900677/4)
---@param playerSrc string
---@param radius number
function SetPlayerCullingRadius(playerSrc, radius) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDFB9A2A2)  
---Make the player impervious to all forms of damage.
---
---**This is the server-side RPC native equivalent of the client native [SET_PLAYER_INVINCIBLE](?\_0x239528EACDC3E7DE).**
---@param player integer
---@param bInvincible boolean
function SetPlayerInvincible(player, bInvincible) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x774A4C54)  
---Set the model for a specific Player. Note that this will destroy the current Ped for the Player and create a new one, any reference to the old ped will be invalid after calling this.
---As per usual, make sure to request the model first and wait until it has loaded.
---
---**This is the server-side RPC native equivalent of the client native [SET_PLAYER_MODEL](?\_0x00A1CADD00108836).**
---@param player integer
---@param model integer | string
function SetPlayerModel(player, model) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6504EB38)  
---Sets the routing bucket for the specified player.
---
---Routing buckets are also known as 'dimensions' or 'virtual worlds' in past echoes, however they are population-aware.
---@param playerSrc string
---@param bucket integer
function SetPlayerRoutingBucket(playerSrc, bucket) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB7A0914B)  
---SET_PLAYER_WANTED_LEVEL
---
---**This is the server-side RPC native equivalent of the client native [SET_PLAYER_WANTED_LEVEL](?\_0x39FF19C64EF7DA5B).**
---@param player integer
---@param wantedLevel integer
---@param delayedResponse boolean
function SetPlayerWantedLevel(player, wantedLevel, delayedResponse) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x21C7A35B)  
---A setter for [GET_RESOURCE_KVP_STRING](#\_0x5240DA5A).
---@param key string
---@param value string
function SetResourceKvp(key, value) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9ADD2938)  
---A setter for [GET_RESOURCE_KVP_FLOAT](#\_0x35BDCEEA).
---@param key string
---@param value number
function SetResourceKvpFloat(key, value) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x3517BFBE)  
---Nonsynchronous [SET_RESOURCE_KVP_FLOAT](#\_0x9ADD2938) operation; see [FLUSH_RESOURCE_KVP](#\_0x5240DA5A).
---@param key string
---@param value number
function SetResourceKvpFloatNoSync(key, value) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6A2B1E8)  
---A setter for [GET_RESOURCE_KVP_INT](#\_0x557B586A).
---@param key string
---@param value integer
function SetResourceKvpInt(key, value) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x26AEB707)  
---Nonsynchronous [SET_RESOURCE_KVP_INT](#\_0x6A2B1E8) operation; see [FLUSH_RESOURCE_KVP](#\_0x5240DA5A).
---@param key string
---@param value integer
function SetResourceKvpIntNoSync(key, value) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xCF9A2FF)  
---Nonsynchronous [SET_RESOURCE_KVP](#\_0x21C7A35B) operation; see [FLUSH_RESOURCE_KVP](#\_0x5240DA5A).
---@param key string
---@param value string
function SetResourceKvpNoSync(key, value) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA0F2201F)  
---Sets the entity lockdown mode for a specific routing bucket.
---
---Lockdown modes are:
---
---| Mode       | Meaning                                                    |
---| ---------- | ---------------------------------------------------------- |
---| `strict`   | No entities can be created by clients at all.              |
---| `relaxed`  | Only script-owned entities created by clients are blocked. |
---| `inactive` | Clients can create any entity they want.                   |
---@param bucketId integer
---@param mode string
function SetRoutingBucketEntityLockdownMode(bucketId, mode) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xCE51AC2C)  
---Sets whether or not the specified routing bucket has automatically-created population enabled.
---@param bucketId integer
---@param mode boolean
function SetRoutingBucketPopulationEnabled(bucketId, mode) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8D50E33A)  
---Internal function for setting a state bag value.
---@param bagName string
---@param keyName string
---@param valueData string
---@param valueLength integer
---@param replicated boolean
function SetStateBagValue(bagName, keyName, valueData, valueLength, replicated) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x24877D84)  
---SET_VEHICLE_ALARM
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_ALARM](?\_0xCDE5E70C1DDB954C).**
---@param vehicle integer
---@param state boolean
function SetVehicleAlarm(vehicle, state) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x920C2517)  
---```
---p2 often set to 1000.0 in the decompiled scripts.
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_BODY_HEALTH](?\_0xB77D05AC8C78AADB).**
---@param vehicle integer
---@param value number
function SetVehicleBodyHealth(vehicle, value) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xA557AEAD)  
---Sets the selected vehicle's colors to their default value (specific variant specified using the colorCombination parameter).
---Range of possible values for colorCombination is currently unknown, I couldn't find where these values are stored either (Disquse's guess was vehicles.meta but I haven't seen it in there.)
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_COLOUR_COMBINATION](?\_0x33E8CD3322E2FE31).**
---@param vehicle integer
---@param colorCombination integer
function SetVehicleColourCombination(vehicle, colorCombination) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x57F24253)  
---colorPrimary & colorSecondary are the paint indexes for the vehicle.
---For a list of valid paint indexes, view: pastebin.com/pwHci0xK
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_COLOURS](?\_0x4F1D4BE3A7F24601).**
---@param vehicle integer
---@param colorPrimary integer
---@param colorSecondary integer
function SetVehicleColours(vehicle, colorPrimary, colorSecondary) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8DF9F9BC)  
---```
---p1, p2, p3 are RGB values for color (255,0,0 for Red, ect)
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_CUSTOM_PRIMARY_COLOUR](?\_0x7141766F91D15BEA).**
---@param vehicle integer
---@param r integer
---@param g integer
---@param b integer
function SetVehicleCustomPrimaryColour(vehicle, r, g, b) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x9D77259E)  
---```
---p1, p2, p3 are RGB values for color (255,0,0 for Red, ect)
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_CUSTOM_SECONDARY_COLOUR](?\_0x36CED73BFED89754).**
---@param vehicle integer
---@param r integer
---@param g integer
---@param b integer
function SetVehicleCustomSecondaryColour(vehicle, r, g, b) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2B39128B)  
---Sets the dirt level of the passed vehicle.
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_DIRT_LEVEL](?\_0x79D3B596FE44EE8B).**
---@param vehicle integer
---@param dirtLevel number
function SetVehicleDirtLevel(vehicle, dirtLevel) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8147FEA7)  
---See eDoorId declared in [`SET_VEHICLE_DOOR_SHUT`](#\_0x93D9BD300D7789E5)
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_DOOR_BROKEN](?\_0xD4D4F6A4AB575A33).**
---@param vehicle integer
---@param doorIndex integer
---@param deleteDoor boolean
function SetVehicleDoorBroken(vehicle, doorIndex, deleteDoor) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x4CDD35D0)  
---Locks the doors of a specified vehicle to a defined lock state, affecting how players and NPCs can interact with the vehicle.
---
---```
---NativeDB Introduced: v323
---```
---
---```cpp
---enum eVehicleLockState {
---// No specific lock state, vehicle behaves according to the game's default settings.
---VEHICLELOCK_NONE = 0,
---// Vehicle is fully unlocked, allowing free entry by players and NPCs.
---VEHICLELOCK_UNLOCKED = 1,
---// Vehicle is locked, preventing entry by players and NPCs.
---VEHICLELOCK_LOCKED = 2,
---// Vehicle locks out only players, allowing NPCs to enter.
---VEHICLELOCK_LOCKOUT_PLAYER_ONLY = 3,
---// Vehicle is locked once a player enters, preventing others from entering.
---VEHICLELOCK_LOCKED_PLAYER_INSIDE = 4,
---// Vehicle starts in a locked state, but may be unlocked through game events.
---VEHICLELOCK_LOCKED_INITIALLY = 5,
---// Forces the vehicle's doors to shut and lock.
---VEHICLELOCK_FORCE_SHUT_DOORS = 6,
---// Vehicle is locked but can still be damaged.
---VEHICLELOCK_LOCKED_BUT_CAN_BE_DAMAGED = 7,
---// Vehicle is locked, but its trunk/boot remains unlocked.
---VEHICLELOCK_LOCKED_BUT_BOOT_UNLOCKED = 8,
---// Vehicle is locked and does not allow passengers, except for the driver.
---VEHICLELOCK_LOCKED_NO_PASSENGERS = 9,
---// Vehicle is completely locked, preventing entry entirely, even if previously inside.
---VEHICLELOCK_CANNOT_ENTER = 10
---};
---```
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_DOORS_LOCKED](?\_0xB664292EAECF7FA6).**
---@param vehicle integer
---@param doorLockStatus integer
function SetVehicleDoorsLocked(vehicle, doorLockStatus) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x400F9556)  
---SET_VEHICLE_NUMBER_PLATE_TEXT
---
---**This is the server-side RPC native equivalent of the client native [SET_VEHICLE_NUMBER_PLATE_TEXT](?\_0x95A88F0B409CDA47).**
---@param vehicle integer
---@param plateText string
function SetVehicleNumberPlateText(vehicle, plateText) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDD379006)  
---This native does not have an official description.
---@param prefix string
---@return integer
function StartFindKvp(prefix) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x29B440DC)  
---This native does not have an official description.
---@param resourceName string
---@return boolean
function StartResource(resourceName) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x12A330)  
---This native does not have an official description.
---@param bagName string
---@param key string
---@return boolean
function StateBagHasKey(bagName, key) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x21783161)  
---This native does not have an official description.
---@param resourceName string
---@return boolean
function StopResource(resourceName) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xCB0D8932)  
---```
---Makes the specified ped attack the target ped.
---p2 should be 0
---p3 should be 16
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_COMBAT_PED](?\_0xF166E48407BAC484).**
---@param ped integer
---@param targetPed integer
---@param p2 integer
---@param p3 integer
function TaskCombatPed(ped, targetPed, p2, p3) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2B84D1C4)  
---```
---Example:
---TASK::TASK_DRIVE_BY(l_467[1/*22*/], PLAYER::PLAYER_PED_ID(), 0, 0.0, 0.0, 2.0, 300.0, 100, 0, ${firing_pattern_burst_fire_driveby});
---Needs working example. Doesn't seem to do anything.
---I marked p2 as targetVehicle as all these shooting related tasks seem to have that in common.
---I marked p6 as distanceToShoot as if you think of GTA's Logic with the native SET_VEHICLE_SHOOT natives, it won't shoot till it gets within a certain distance of the target.
---I marked p7 as pedAccuracy as it seems it's mostly 100 (Completely Accurate), 75, 90, etc. Although this could be the ammo count within the gun, but I highly doubt it. I will change this comment once I find out if it's ammo count or not.
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_DRIVE_BY](?\_0x2F8AF0E82773A171).**
---@param driverPed integer
---@param targetPed integer
---@param targetVehicle integer
---@param targetX number
---@param targetY number
---@param targetZ number
---@param distanceToShoot number
---@param pedAccuracy integer
---@param p8 boolean
---@param firingPattern integer | string
function TaskDriveBy(driverPed, targetPed, targetVehicle, targetX, targetY, targetZ, distanceToShoot, pedAccuracy, p8, firingPattern) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xB8689B4E)  
---```
---speed 1.0 = walk, 2.0 = run
---p5 1 = normal, 3 = teleport to vehicle, 8 = normal/carjack ped from seat, 16 = teleport directly into vehicle
---p6 is always 0
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_ENTER_VEHICLE](?\_0xC20E50AA46D09CA8).**
---@param ped integer
---@param vehicle integer
---@param timeout integer
---@param seatIndex integer
---@param speed number
---@param flag integer
---@param p6 any
function TaskEnterVehicle(ped, vehicle, timeout, seatIndex, speed, flag, p6) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xC1971F30)  
---TASK_EVERYONE_LEAVE_VEHICLE
---
---**This is the server-side RPC native equivalent of the client native [TASK_EVERYONE_LEAVE_VEHICLE](?\_0x7F93691AB4B92272).**
---@param vehicle integer
function TaskEveryoneLeaveVehicle(vehicle) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x80A9E7A7)  
---TASK_GO_STRAIGHT_TO_COORD
---
---**This is the server-side RPC native equivalent of the client native [TASK_GO_STRAIGHT_TO_COORD](?\_0xD76B57B44F1E6F8B).**
---@param ped integer
---@param x number
---@param y number
---@param z number
---@param speed number
---@param timeout integer
---@param targetHeading number
---@param distanceToSlide number
function TaskGoStraightToCoord(ped, x, y, z, speed, timeout, targetHeading, distanceToSlide) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xF91DF93B)  
---Tells a ped to go to a coord by any means.
---
---```cpp
---enum eDrivingMode {
---DF_StopForCars = 1,
---DF_StopForPeds = 2,
---DF_SwerveAroundAllCars = 4,
---DF_SteerAroundStationaryCars = 8,
---DF_SteerAroundPeds = 16,
---DF_SteerAroundObjects = 32,
---DF_DontSteerAroundPlayerPed = 64,
---DF_StopAtLights = 128,
---DF_GoOffRoadWhenAvoiding = 256,
---DF_DriveIntoOncomingTraffic = 512,
---DF_DriveInReverse = 1024,
---// If pathfinding fails, cruise randomly instead of going on a straight line
---DF_UseWanderFallbackInsteadOfStraightLine = 2048,
---DF_AvoidRestrictedAreas = 4096,
---// These only work on MISSION_CRUISE
---DF_PreventBackgroundPathfinding = 8192,
---DF_AdjustCruiseSpeedBasedOnRoadSpeed = 16384,
---DF_UseShortCutLinks =  262144,
---DF_ChangeLanesAroundObstructions = 524288,
---// cruise tasks ignore this anyway--only used for goto's
---DF_UseSwitchedOffNodes =  2097152,
---// if you're going to be primarily driving off road
---DF_PreferNavmeshRoute =  4194304,
---// Only works for planes using MISSION_GOTO, will cause them to drive along the ground instead of fly
---DF_PlaneTaxiMode =  8388608,
---DF_ForceStraightLine = 16777216,
---DF_UseStringPullingAtJunctions = 33554432,
---DF_AvoidHighways = 536870912,
---DF_ForceJoinInRoadDirection = 1073741824,
---// Standard driving mode. stops for cars, peds, and lights, goes around stationary obstructions
---DRIVINGMODE_STOPFORCARS = 786603, // DF_StopForCars|DF_StopForPeds|DF_SteerAroundObjects|DF_SteerAroundStationaryCars|DF_StopAtLights|DF_UseShortCutLinks|DF_ChangeLanesAroundObstructions,		// Obey lights too
---// Like the above, but doesn't steer around anything in its way - will only wait instead.
---DRIVINGMODE_STOPFORCARS_STRICT = 262275, // DF_StopForCars|DF_StopForPeds|DF_StopAtLights|DF_UseShortCutLinks, // Doesn't deviate an inch.
---// Default "alerted" driving mode. drives around everything, doesn't obey lights
---DRIVINGMODE_AVOIDCARS = 786469, // DF_SwerveAroundAllCars|DF_SteerAroundObjects|DF_UseShortCutLinks|DF_ChangeLanesAroundObstructions|DF_StopForCars,
---// Very erratic driving. difference between this and AvoidCars is that it doesn't use the brakes at ALL to help with steering
---DRIVINGMODE_AVOIDCARS_RECKLESS = 786468, // DF_SwerveAroundAllCars|DF_SteerAroundObjects|DF_UseShortCutLinks|DF_ChangeLanesAroundObstructions,
---// Smashes through everything
---DRIVINGMODE_PLOUGHTHROUGH = 262144, // DF_UseShortCutLinks
---// Drives normally except for the fact that it ignores lights
---DRIVINGMODE_STOPFORCARS_IGNORELIGHTS = 786475, // DF_StopForCars|DF_SteerAroundStationaryCars|DF_StopForPeds|DF_SteerAroundObjects|DF_UseShortCutLinks|DF_ChangeLanesAroundObstructions
---// Try to swerve around everything, but stop for lights if necessary
---DRIVINGMODE_AVOIDCARS_OBEYLIGHTS = 786597, // DF_SwerveAroundAllCars|DF_StopAtLights|DF_SteerAroundObjects|DF_UseShortCutLinks|DF_ChangeLanesAroundObstructions|DF_StopForCars
---// Swerve around cars, be careful around peds, and stop for lights
---DRIVINGMODE_AVOIDCARS_STOPFORPEDS_OBEYLIGHTS = 786599 // DF_SwerveAroundAllCars|DF_StopAtLights|DF_StopForPeds|DF_SteerAroundObjects|DF_UseShortCutLinks|DF_ChangeLanesAroundObstructions|DF_StopForCars
---};
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_GO_TO_COORD_ANY_MEANS](?\_0x5BC448CB78FA3E88).**
---@param ped integer
---@param x number
---@param y number
---@param z number
---@param fMoveBlendRatio number
---@param vehicle integer
---@param bUseLongRangeVehiclePathing boolean
---@param drivingFlags integer
---@param fMaxRangeToShootTargets number
function TaskGoToCoordAnyMeans(ped, x, y, z, fMoveBlendRatio, vehicle, bUseLongRangeVehiclePathing, drivingFlags, fMaxRangeToShootTargets) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x374827C2)  
---```
---The entity will move towards the target until time is over (duration) or get in target's range (distance). p5 and p6 are unknown, but you could leave p5 = 1073741824 or 100 or even 0 (didn't see any difference but on the decompiled scripts, they use 1073741824 mostly) and p6 = 0
---Note: I've only tested it on entity -> ped and target -> vehicle. It could work differently on other entities, didn't try it yet.
---Example: TASK::TASK_GO_TO_ENTITY(pedHandle, vehicleHandle, 5000, 4.0, 100, 1073741824, 0)
---Ped will run towards the vehicle for 5 seconds and stop when time is over or when he gets 4 meters(?) around the vehicle (with duration = -1, the task duration will be ignored).
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_GO_TO_ENTITY](?\_0x6A071245EB0D1882).**
---@param entity integer
---@param target integer
---@param duration integer
---@param distance number
---@param speed number
---@param p5 number
---@param p6 integer
function TaskGoToEntity(entity, target, duration, distance, speed, p5, p6) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8DCC19C5)  
---```
---In the scripts, p3 was always -1.
---p3 seems to be duration or timeout of turn animation.
---Also facingPed can be 0 or -1 so ped will just raise hands up.
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_HANDS_UP](?\_0xF2EAB31979A7F910).**
---@param ped integer
---@param duration integer
---@param facingPed integer
---@param p3 integer
---@param p4 boolean
function TaskHandsUp(ped, duration, facingPed, p3, p4) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xDBDD79FA)  
---Flags are the same flags used in [`TASK_LEAVE_VEHICLE`](#\_0xD3DBCE61A490BE02)
---
---**This is the server-side RPC native equivalent of the client native [TASK_LEAVE_ANY_VEHICLE](?\_0x504D54DF3F6F2247).**
---@param ped integer
---@param p1 integer
---@param flags integer
function TaskLeaveAnyVehicle(ped, p1, flags) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x7B1141C6)  
---```
---Flags from decompiled scripts:
---0 = normal exit and closes door.
---1 = normal exit and closes door.
---16 = teleports outside, door kept closed.  (This flag does not seem to work for the front seats in buses, NPCs continue to exit normally)
---64 = normal exit and closes door, maybe a bit slower animation than 0.
---256 = normal exit but does not close the door.
---4160 = ped is throwing himself out, even when the vehicle is still.
---262144 = ped moves to passenger seat first, then exits normally
---Others to be tried out: 320, 512, 131072.
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_LEAVE_VEHICLE](?\_0xD3DBCE61A490BE02).**
---@param ped integer
---@param vehicle integer
---@param flags integer
function TaskLeaveVehicle(ped, vehicle, flags) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x5AB552C6)  
---[Animations list](https://alexguirre.github.io/animations-list/)
---
---```cpp
---enum eScriptedAnimFlags
---{
---AF_LOOPING = 1,
---AF_HOLD_LAST_FRAME = 2,
---AF_REPOSITION_WHEN_FINISHED = 4,
---AF_NOT_INTERRUPTABLE = 8,
---AF_UPPERBODY = 16,
---AF_SECONDARY = 32,
---AF_REORIENT_WHEN_FINISHED = 64,
---AF_ABORT_ON_PED_MOVEMENT = 128,
---AF_ADDITIVE = 256,
---AF_TURN_OFF_COLLISION = 512,
---AF_OVERRIDE_PHYSICS = 1024,
---AF_IGNORE_GRAVITY = 2048,
---AF_EXTRACT_INITIAL_OFFSET = 4096,
---AF_EXIT_AFTER_INTERRUPTED = 8192,
---AF_TAG_SYNC_IN = 16384,
---AF_TAG_SYNC_OUT = 32768,
---AF_TAG_SYNC_CONTINUOUS = 65536,
---AF_FORCE_START = 131072,
---AF_USE_KINEMATIC_PHYSICS = 262144,
---AF_USE_MOVER_EXTRACTION = 524288,
---AF_HIDE_WEAPON = 1048576,
---AF_ENDS_IN_DEAD_POSE = 2097152,
---AF_ACTIVATE_RAGDOLL_ON_COLLISION = 4194304,
---AF_DONT_EXIT_ON_DEATH = 8388608,
---AF_ABORT_ON_WEAPON_DAMAGE = 16777216,
---AF_DISABLE_FORCED_PHYSICS_UPDATE = 33554432,
---AF_PROCESS_ATTACHMENTS_ON_START = 67108864,
---AF_EXPAND_PED_CAPSULE_FROM_SKELETON = 134217728,
---AF_USE_ALTERNATIVE_FP_ANIM = 268435456,
---AF_BLENDOUT_WRT_LAST_FRAME = 536870912,
---AF_USE_FULL_BLENDING = 1073741824
---}
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_PLAY_ANIM](?\_0xEA47FE3719165B94).**
---@param ped integer
---@param animDictionary string
---@param animationName string
---@param blendInSpeed number
---@param blendOutSpeed number
---@param duration integer
---@param flag integer
---@param playbackRate number
---@param lockX boolean
---@param lockY boolean
---@param lockZ boolean
function TaskPlayAnim(ped, animDictionary, animationName, blendInSpeed, blendOutSpeed, duration, flag, playbackRate, lockX, lockY, lockZ) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x3DDEB0E6)  
---Similar in functionality to [`TASK_PLAY_ANIM`](#\_0xEA47FE3719165B94), except the position and rotation parameters let you specify the initial position and rotation of the task. The ped is teleported to the position specified.
---[Animations list](https://alexguirre.github.io/animations-list/)
---
---**This is the server-side RPC native equivalent of the client native [TASK_PLAY_ANIM_ADVANCED](?\_0x83CDB10EA29B370B).**
---@param ped integer
---@param animDictionary string
---@param animationName string
---@param posX number
---@param posY number
---@param posZ number
---@param rotX number
---@param rotY number
---@param rotZ number
---@param blendInSpeed number
---@param blendOutSpeed number
---@param duration integer
---@param flag any
---@param animTime number
---@param p14 any
---@param p15 any
function TaskPlayAnimAdvanced(ped, animDictionary, animationName, posX, posY, posZ, rotX, rotY, rotZ, blendInSpeed, blendOutSpeed, duration, flag, animTime, p14, p15) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x8A632BD8)  
---TASK_REACT_AND_FLEE_PED
---
---**This is the server-side RPC native equivalent of the client native [TASK_REACT_AND_FLEE_PED](?\_0x72C896464915D1B1).**
---@param ped integer
---@param fleeTarget integer
function TaskReactAndFleePed(ped, fleeTarget) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x601C22E3)  
---```
---Firing Pattern Hash Information: https://pastebin.com/Px036isB
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_SHOOT_AT_COORD](?\_0x46A6CC01E0826106).**
---@param ped integer
---@param x number
---@param y number
---@param z number
---@param duration integer
---@param firingPattern integer | string
function TaskShootAtCoord(ped, x, y, z, duration, firingPattern) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xAC0631C9)  
---```
---//this part of the code is to determine at which entity the player is aiming, for example if you want to create a mod where you give orders to peds
---Entity aimedentity;
---Player player = PLAYER::PLAYER_ID();
---PLAYER::_GET_AIMED_ENTITY(player, &aimedentity);
---//bg is an array of peds
---TASK::TASK_SHOOT_AT_ENTITY(bg[i], aimedentity, 5000, MISC::GET_HASH_KEY("FIRING_PATTERN_FULL_AUTO"));
---in practical usage, getting the entity the player is aiming at and then task the peds to shoot at the entity, at a button press event would be better.
---Firing Pattern Hash Information: https://pastebin.com/Px036isB
---```
---
---**This is the server-side RPC native equivalent of the client native [TASK_SHOOT_AT_ENTITY](?\_0x08DA95E8298AE772).**
---@param entity integer
---@param target integer
---@param duration integer
---@param firingPattern integer | string
function TaskShootAtEntity(entity, target, duration, firingPattern) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x65D4A35D)  
---```
---NativeDB Introduced: v323
---```
---
---Warp a ped into a vehicle.
---**Note**: It's better to use [`TASK_ENTER_VEHICLE`](#\_0xC20E50AA46D09CA8) with the flag "warp" flag instead of this native.
---
---**This is the server-side RPC native equivalent of the client native [TASK_WARP_PED_INTO_VEHICLE](?\_0x9A7D091411C5F684).**
---@param ped integer
---@param vehicle integer
---@param seatIndex integer
function TaskWarpPedIntoVehicle(ped, vehicle, seatIndex) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x1E35DBBA)  
---This native does not have an official description.
---@param playerSrc string
---@param reason string
function TempBanPlayer(playerSrc, reason) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2F7A49E6)  
---The backing function for TriggerClientEvent.
---@param eventName string
---@param eventTarget string
---@param eventPayload string
---@param payloadLength integer
function TriggerClientEventInternal(eventName, eventTarget, eventPayload, payloadLength) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x91310870)  
---The backing function for TriggerEvent.
---@param eventName string
---@param eventPayload string
---@param payloadLength integer
function TriggerEventInternal(eventName, eventPayload, payloadLength) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x70B35890)  
---The backing function for TriggerLatentClientEvent.
---@param eventName string
---@param eventTarget string
---@param eventPayload string
---@param payloadLength integer
---@param bps integer
function TriggerLatentClientEventInternal(eventName, eventTarget, eventPayload, payloadLength, bps) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x2E310ACD)  
---This native does not have an official description.
---@param password string
---@param hash string
---@return boolean
function VerifyPasswordHash(password, hash) end

---**`CFX` `shared`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x58382A19)  
---Returns whether or not the currently executing event was canceled.
---@return boolean
function WasEventCanceled() end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6228F159)  
---Adds a rectangular blip for the specified coordinates/area.
---It is recommended to use [SET_BLIP_ROTATION](#\_0xF87683CDF73C3F6E) and [SET_BLIP_COLOUR](#\_0x03D7FB09E75D6B7E) to make the blip not rotate along with the camera.
---By default, the blip will show as a *regular* blip with the specified color/sprite if it is outside of the minimap view.
---Example image:
---![minimap](https://i.imgur.com/qLbXWcQ.png)
---![big map](https://i.imgur.com/0j7O7Rh.png)
---(Native name is *likely* to actually be ADD_BLIP_FOR_AREA, but due to the usual reasons this can't be confirmed)
---
---**This is the server-side RPC native equivalent of the client native [\_ADD_BLIP_FOR_AREA](?\_0xCE5D0E5E315DB238).**
---@param x number
---@param y number
---@param z number
---@param width number
---@param height number
---@return integer
function _addBlipForArea(x, y, z, width, height) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0xEC09DB1B)  
---Used for freemode (online) characters.
---Indices:
---
---1.  black
---2.  very light blue/green
---3.  dark blue
---4.  brown
---5.  darker brown
---6.  light brown
---7.  blue
---8.  light blue
---9.  pink
---10. yellow
---11. purple
---12. black
---13. dark green
---14. light brown
---15. yellow/black pattern
---16. light colored spiral pattern
---17. shiny red
---18. shiny half blue/half red
---19. half black/half light blue
---20. white/red perimter
---21. green snake
---22. red snake
---23. dark blue snake
---24. dark yellow
---25. bright yellow
---26. all black
---27. red small pupil
---28. devil blue/black
---29. white small pupil
---30. glossed over
---
---**This is the server-side RPC native equivalent of the client native [\_SET_PED_EYE_COLOR](?\_0x50B56988B170AFDF).**
---@param ped integer
---@param index integer
function _setPedEyeColor(ped, index) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x6C8D4458)  
---Sets the various freemode face features, e.g. nose length, chin shape.
---**Indexes (From 0 to 19):**
---Parentheses indicate morph scale/direction as in (-1.0 to 1.0)
---
---*   **0**: Nose Width (Thin/Wide)
---*   **1**: Nose Peak (Up/Down)
---*   **2**: Nose Length (Long/Short)
---*   **3**: Nose Bone Curveness (Crooked/Curved)
---*   **4**: Nose Tip (Up/Down)
---*   **5**: Nose Bone Twist (Left/Right)
---*   **6**: Eyebrow (Up/Down)
---*   **7**: Eyebrow (In/Out)
---*   **8**: Cheek Bones (Up/Down)
---*   **9**: Cheek Sideways Bone Size (In/Out)
---*   **10**: Cheek Bones Width (Puffed/Gaunt)
---*   **11**: Eye Opening (Both) (Wide/Squinted)
---*   **12**: Lip Thickness (Both) (Fat/Thin)
---*   **13**: Jaw Bone Width (Narrow/Wide)
---*   **14**: Jaw Bone Shape (Round/Square)
---*   **15**: Chin Bone (Up/Down)
---*   **16**: Chin Bone Length (In/Out or Backward/Forward)
---*   **17**: Chin Bone Shape (Pointed/Square)
---*   **18**: Chin Hole (Chin Bum)
---*   **19**: Neck Thickness (Thin/Thick)
---    **Note:**
---    You may need to call [`SetPedHeadBlendData`](#\_0x9414E18B9434C2FE) prior to calling this native in order for it to work.
---
---**This is the server-side RPC native equivalent of the client native [\_SET_PED_FACE_FEATURE](?\_0x71A5C1DBA060049E).**
---@param ped integer
---@param index integer
---@param scale number
function _setPedFaceFeature(ped, index, scale) end

---**`CFX` `server`**  
---[Native Documentation](https://docs.fivem.net/natives/?_0x78935A27)  
---```
---Used for freemode (online) characters.
---Called after SET_PED_HEAD_OVERLAY().
---```
---
---**Note:**
---You may need to call [`SetPedHeadBlendData`](#\_0x9414E18B9434C2FE) prior to calling this native in order for it to work.
---
---**This is the server-side RPC native equivalent of the client native [\_SET_PED_HEAD_OVERLAY_COLOR](?\_0x497BF74A7B9CB952).**
---@param ped integer
---@param overlayID integer
---@param colorType integer
---@param colorID integer
---@param secondColorID integer
function _setPedHeadOverlayColor(ped, overlayID, colorType, colorID, secondColorID) end
