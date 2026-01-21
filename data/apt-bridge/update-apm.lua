local gettext_handle = io.popen("gettext apm 'Updating apm database' 2>/dev/null")
local message = gettext_handle:read("*l")
gettext_handle:close()

if not message or message == "" then
    message = "Updating apm database"
end

print(message)
os.execute("apm s update --no-lock")
