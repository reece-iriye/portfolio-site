local _M = {}

local function init(dict_name)
	local self = {
		dict_name = dict_name or "prometheus_metrics",
		dict = ngx.shared[dict_name or "prometheus_metrics"],
		metrics = {},
	}

	if not self.dict then
		ngx.log(ngx.ERR, "Dictionary " .. self.dict_name .. " not found")
		return nil
	end

	setmetatable(self, { __index = _M })
	return self
end

function _M:counter(name, help, labels)
	local metric = {
		type = "counter",
		name = name,
		help = help,
		labels = labels or {},
		prom = self,
	}

	function metric:inc(value, label_values)
		value = value or 1
		local key = self.name
		if label_values and #label_values > 0 then
			key = key .. "{" .. self:format_labels(label_values) .. "}"
		end

		local current = self.prom.dict:get(key) or 0
		local ok, err = self.prom.dict:set(key, current + value)
		if not ok then
			ngx.log(ngx.ERR, "Failed to increment counter: " .. (err or "unknown error"))
		end
	end

	function metric:format_labels(values)
		local parts = {}
		for i, label in ipairs(self.labels) do
			if values[i] then
				table.insert(parts, label .. '="' .. tostring(values[i]):gsub('"', '\\"') .. '"')
			end
		end
		return table.concat(parts, ",")
	end

	self.metrics[name] = metric
	return metric
end

function _M:histogram(name, help, labels, buckets)
	buckets = buckets or { 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10 }

	local metric = {
		type = "histogram",
		name = name,
		help = help,
		labels = labels or {},
		buckets = buckets,
		prom = self,
	}

	function metric:observe(value, label_values)
		local base_key = self.name
		local label_str = ""
		if label_values and #label_values > 0 then
			label_str = "{" .. self:format_labels(label_values) .. "}"
		end

		-- Count bucket
		for _, bucket in ipairs(self.buckets) do
			if value <= bucket then
				local bucket_key = base_key .. "_bucket" .. label_str:gsub("}", ',le="' .. bucket .. '"}')
				if label_str == "" then
					bucket_key = base_key .. '_bucket{le="' .. bucket .. '"}'
				end
				local current = self.prom.dict:get(bucket_key) or 0
				self.prom.dict:set(bucket_key, current + 1)
			end
		end

		-- +Inf bucket
		local inf_key = base_key .. "_bucket" .. label_str:gsub("}", ',le="+Inf"}')
		if label_str == "" then
			inf_key = base_key .. '_bucket{le="+Inf"}'
		end
		local current_inf = self.prom.dict:get(inf_key) or 0
		self.prom.dict:set(inf_key, current_inf + 1)

		-- Count total
		local count_key = base_key .. "_count" .. label_str
		local current_count = self.prom.dict:get(count_key) or 0
		self.prom.dict:set(count_key, current_count + 1)

		-- Sum
		local sum_key = base_key .. "_sum" .. label_str
		local current_sum = self.prom.dict:get(sum_key) or 0
		self.prom.dict:set(sum_key, current_sum + value)
	end

	function metric:format_labels(values)
		local parts = {}
		for i, label in ipairs(self.labels) do
			if values[i] then
				table.insert(parts, label .. '="' .. tostring(values[i]):gsub('"', '\\"') .. '"')
			end
		end
		return table.concat(parts, ",")
	end

	self.metrics[name] = metric
	return metric
end

function _M:gauge(name, help, labels)
	local metric = {
		type = "gauge",
		name = name,
		help = help,
		labels = labels or {},
		prom = self,
	}

	function metric:set(value, label_values)
		local key = self.name
		if label_values and #label_values > 0 then
			key = key .. "{" .. self:format_labels(label_values) .. "}"
		end

		local ok, err = self.prom.dict:set(key, value)
		if not ok then
			ngx.log(ngx.ERR, "Failed to set gauge: " .. (err or "unknown error"))
		end
	end

	function metric:inc(value, label_values)
		value = value or 1
		local key = self.name
		if label_values and #label_values > 0 then
			key = key .. "{" .. self:format_labels(label_values) .. "}"
		end

		local current = self.prom.dict:get(key) or 0
		local ok, err = self.prom.dict:set(key, current + value)
		if not ok then
			ngx.log(ngx.ERR, "Failed to increment gauge: " .. (err or "unknown error"))
		end
	end

	function metric:format_labels(values)
		local parts = {}
		for i, label in ipairs(self.labels) do
			if values[i] then
				table.insert(parts, label .. '="' .. tostring(values[i]):gsub('"', '\\"') .. '"')
			end
		end
		return table.concat(parts, ",")
	end

	self.metrics[name] = metric
	return metric
end

function _M:collect()
	ngx.header.content_type = "text/plain; version=0.0.4; charset=utf-8"

	-- Get all keys from the dictionary
	local keys = self.dict:get_keys(0)
	local output = {}
	local help_output = {}
	local type_output = {}

	-- Group metrics by name for proper output format
	local metric_groups = {}

	for _, key in ipairs(keys) do
		local value = self.dict:get(key)
		if value then
			local base_name = key:match("^([^{_]+)")
			if not metric_groups[base_name] then
				metric_groups[base_name] = {}
			end
			table.insert(metric_groups[base_name], { key = key, value = value })
		end
	end

	-- Output metrics
	for name, metrics in pairs(metric_groups) do
		local metric_info = self.metrics[name]
		if metric_info then
			table.insert(help_output, "# HELP " .. name .. " " .. (metric_info.help or ""))
			table.insert(type_output, "# TYPE " .. name .. " " .. metric_info.type)
		end

		for _, metric in ipairs(metrics) do
			table.insert(output, metric.key .. " " .. metric.value)
		end
	end

	-- Combine all output
	local result = {}
	for _, line in ipairs(help_output) do
		table.insert(result, line)
	end
	for _, line in ipairs(type_output) do
		table.insert(result, line)
	end
	for _, line in ipairs(output) do
		table.insert(result, line)
	end

	ngx.say(table.concat(result, "\n"))
end

_M.init = init
return _M
