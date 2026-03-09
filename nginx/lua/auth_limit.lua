---@diagnostic disable: undefined-global
local ngx = ngx
local cjson = require "cjson"
local auth_header = ngx.var.http_authorization

if not auth_header then
    return -- Нет заголовка — выходим (будет применен стандартный лимит по IP)
end

-- Извлекаем токен
local _, _, token = string.find(auth_header, "Bearer%s+(.+)")
if not token then return end

-- Парсим Payload (вторая часть JWT)
local parts = {}
for part in string.gmatch(token, "[^.]+") do
    table.insert(parts, part)
end

if #parts >= 2 then
    local payload_b64 = parts[2]
    -- Заменяем base64url символы на обычный base64
    payload_b64 = string.gsub(payload_b64, "-", "+")
    payload_b64 = string.gsub(payload_b64, "_", "/")

    local payload = ngx.decode_base64(payload_b64)
    if payload then
        local ok, data = pcall(cjson.decode, payload)
        if ok then
            -- Записываем данные в переменные Nginx
            ngx.var.user_id = data["user_id"] or ""
            ngx.var.user_role = data["role"] or "user"
        end
    end
end

-- Если это АДМИН — отключаем лимит запросов для текущего запроса
if ngx.var.user_role == "admin" then
    -- Мы просто не выставляем user_id, или выставляем пустую строку,
    -- если зона лимита настроена на пропуск пустых ключей
    ngx.var.user_id = ""
end

local uri = ngx.var.uri
local args = ngx.req.get_uri_args()

for k, v in pairs(args) do
    if type(v) == "string" then
        -- Ищем типичные паттерны атак (кавычки, комментарии, союзы)
        if string.find(v, "['\";%-%-/%*]") or string.find(v:lower(), "union") then
            ngx.log(ngx.WARN, "ПОПЫТКА ИНЪЕКЦИИ: ", v)
            return ngx.exit(403) -- Просто выкидываем его
        end
    end
end
