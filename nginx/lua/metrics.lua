local _M = {}

local math_floor = math.floor
local string_format = string.format
local table_concat = table.concat
local table_insert = table.insert

function _M.calculate_percentile(buckets, counts, percentile)
	if not buckets or not counts or #buckets ~= #counts then
		return 0
	end

	local total = 0
	for i = 1, #counts do
		total = total + counts[i]
	end

	if total == 0 then
		return 0
	end

	local target = total * percentile
	local current = 0

	for i = 1, #buckets do
		current = current + counts[i]
		if current >= target then
			if i == 1 then
				return buckets[1]
			else
				-- Linear interpolation between buckets
				local prev_count = current - counts[i]
				local bucket_range = buckets[i] - buckets[i - 1]
				local position = (target - prev_count) / counts[i]
				return buckets[i - 1] + (bucket_range * position)
			end
		end
	end

	return buckets[#buckets]
end

function _M.categorize_request(uri, method)
	if not uri then
		return "unknown"
	end

	local api_patterns = {
		{ pattern = "^/api/home$", category = "/api/home" },
		{ pattern = "^/api/work-history$", category = "/api/work-history" },
		{ pattern = "^/api/contact-me$", category = "/api/contact-me" },
		{ pattern = "^/api/contact$", category = "/api/contact" },
		{ pattern = "^/api/", category = "/api/*" },
	}

	for _, rule in ipairs(api_patterns) do
		if uri:match(rule.pattern) then
			return rule.category
		end
	end

	local static_patterns = {
		{ pattern = "^/assets/", category = "/assets/*" },
		{ pattern = "^/static/", category = "/static/*" },
		{ pattern = "^/uploads/", category = "/uploads/*" },
		{ pattern = "^/images/", category = "/images/*" },
		{ pattern = "^/css/", category = "/css/*" },
		{ pattern = "^/js/", category = "/js/*" },
	}

	for _, rule in ipairs(static_patterns) do
		if uri:match(rule.pattern) then
			return rule.category
		end
	end

	local admin_patterns = {
		{ pattern = "^/admin/", category = "/admin/*" },
		{ pattern = "^/dashboard/", category = "/dashboard/*" },
		{ pattern = "^/management/", category = "/management/*" },
	}

	for _, rule in ipairs(admin_patterns) do
		if uri:match(rule.pattern) then
			return rule.category
		end
	end

	-- Health check and monitoring
	if uri:match("^/health") or uri:match("^/ping") or uri:match("^/status") then
		return "/health"
	end

	-- Root and common pages
	if uri == "/" then
		return "/"
	elseif uri:match("^/about") then
		return "/about"
	elseif uri:match("^/contact") then
		return "/contact"
	end

	return "/other"
end

function _M.timing_bucket(duration)
	if not duration or type(duration) ~= "number" then
		return "unknown"
	end

	if duration <= 0.001 then
		return "ultra-fast"
	elseif duration <= 0.01 then
		return "very-fast"
	elseif duration <= 0.1 then
		return "fast"
	elseif duration <= 0.5 then
		return "normal"
	elseif duration <= 1.0 then
		return "slow"
	elseif duration <= 5.0 then
		return "very-slow"
	else
		return "timeout"
	end
end

function _M.size_bucket(bytes)
	if not bytes or type(bytes) ~= "number" then
		return "unknown"
	end

	if bytes <= 1024 then
		return "tiny" -- < 1KB
	elseif bytes <= 10240 then
		return "small" -- < 10KB
	elseif bytes <= 102400 then
		return "medium" -- < 100KB
	elseif bytes <= 1048576 then
		return "large" -- < 1MB
	elseif bytes <= 10485760 then
		return "xlarge" -- < 10MB
	else
		return "huge" -- > 10MB
	end
end

function _M.status_category(status_code)
	if not status_code then
		return "unknown"
	end

	local code = tonumber(status_code)
	if not code then
		return "invalid"
	end

	if code >= 200 and code < 300 then
		return "success"
	elseif code >= 300 and code < 400 then
		return "redirect"
	elseif code >= 400 and code < 500 then
		return "client-error"
	elseif code >= 500 then
		return "server-error"
	else
		return "informational"
	end
end

function _M.parse_user_agent(ua)
	if not ua or ua == "" then
		return { browser = "unknown", os = "unknown", device = "unknown" }
	end

	local result = { browser = "unknown", os = "unknown", device = "unknown" }

	-- Browser detection
	if ua:match("Chrome/") and not ua:match("Edg/") then
		result.browser = "Chrome"
	elseif ua:match("Firefox/") then
		result.browser = "Firefox"
	elseif ua:match("Safari/") and not ua:match("Chrome/") then
		result.browser = "Safari"
	elseif ua:match("Edg/") then
		result.browser = "Edge"
	elseif ua:match("Opera/") or ua:match("OPR/") then
		result.browser = "Opera"
	elseif ua:match("bot") or ua:match("Bot") or ua:match("crawler") or ua:match("spider") then
		result.browser = "bot"
	end

	-- OS detection
	if ua:match("Windows NT") then
		result.os = "Windows"
	elseif ua:match("Mac OS X") then
		result.os = "macOS"
	elseif ua:match("Linux") and not ua:match("Android") then
		result.os = "Linux"
	elseif ua:match("Android") then
		result.os = "Android"
	elseif ua:match("iPhone OS") or ua:match("iOS") then
		result.os = "iOS"
	end

	-- Device detection
	if ua:match("Mobile") or ua:match("Android") or ua:match("iPhone") then
		result.device = "mobile"
	elseif ua:match("Tablet") or ua:match("iPad") then
		result.device = "tablet"
	else
		result.device = "desktop"
	end

	return result
end

function _M.check_rate_limit(key, limit, window, dict_name)
	local dict = ngx.shared[dict_name or "rate_limit"]
	if not dict then
		ngx.log(ngx.ERR, "Rate limit dictionary not found: ", dict_name or "rate_limit")
		return true -- Allow by default if dict not found
	end

	local current_time = ngx.time()
	local window_start = math_floor(current_time / window) * window
	local rate_key = key .. ":" .. window_start

	local current = dict:get(rate_key) or 0
	if current >= limit then
		return false
	end

	-- Use incr with expiry
	local new_val, err = dict:incr(rate_key, 1, 0, window + 1)
	if not new_val then
		ngx.log(ngx.ERR, "Failed to increment rate limit counter: ", err)
		return true -- Allow by default on error
	end

	return true
end

function _M.safe_json_encode(data)
	-- Try to load cjson first
	local has_cjson, cjson = pcall(require, "cjson")
	if has_cjson then
		local ok, result = pcall(cjson.encode, data)
		if ok then
			return result
		end
	end

	-- Fallback to simple string formatting
	if type(data) == "table" then
		local parts = {}
		for k, v in pairs(data) do
			local key_str = tostring(k):gsub('"', '\\"')
			local val_str = tostring(v):gsub('"', '\\"')
			table_insert(parts, string_format('"%s":"%s"', key_str, val_str))
		end
		return "{" .. table_concat(parts, ",") .. "}"
	else
		local safe_str = tostring(data):gsub('"', '\\"')
		return string_format('"%s"', safe_str)
	end
end

function _M.get_memory_usage()
	local result = {
		lua_memory = collectgarbage("count") * 1024,
		version = _VERSION,
	}

	if jit then
		result.jit_version = jit.version
		result.jit_enabled = jit.status()
	end

	return result
end

function _M.safe_log(level, message, context)
	local log_levels = {
		debug = ngx.DEBUG,
		info = ngx.INFO,
		warn = ngx.WARN,
		error = ngx.ERR,
	}

	local log_level = log_levels[level] or ngx.INFO
	local log_message = tostring(message or "")

	if context then
		log_message = log_message .. " | Context: " .. _M.safe_json_encode(context)
	end

	ngx.log(log_level, log_message)
end

-- Utility function to sanitize strings for Prometheus labels
function _M.sanitize_label_value(value)
	if not value then
		return "unknown"
	end

	local str = tostring(value)
	-- Remove or replace problematic characters
	str = str:gsub('[%c\r\n\t"]', "_") -- Replace control chars and quotes
	str = str:gsub("^%s+", ""):gsub("%s+$", "") -- Trim whitespace

	if str == "" then
		return "unknown"
	end

	return str
end

-- Function to validate metric names
function _M.validate_metric_name(name)
	if not name or type(name) ~= "string" then
		return false, "Metric name must be a string"
	end

	if not name:match("^[a-zA-Z_:][a-zA-Z0-9_:]*$") then
		return false, "Invalid metric name format"
	end

	return true
end

return _M
