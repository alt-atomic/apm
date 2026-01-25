local function is_root()
    local handle = io.popen("whoami 2>/dev/null")
    if not handle then
        return false
    end
    local user = handle:read("*l")
    handle:close()
    return user == "root"
end

if not is_root() then
    os.exit(0)
end

local gettext_handle = io.popen("gettext apm 'Updating apm database' 2>/dev/null")
local message = gettext_handle:read("*l")
gettext_handle:close()

if not message or message == "" then
    message = "Updating apm database"
end

print(message)
os.execute("apm s update --no-lock")
