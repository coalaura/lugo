PLAIN_ROOT = true

local function plain_helper()
	return --[[@plain_exports]]exports, --[[@plain_source]]source
end

return plain_helper()
