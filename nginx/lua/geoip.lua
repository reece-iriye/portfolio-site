local _M = {}

function _M.init()
	ngx.log(ngx.INFO, "Cloudflare GeoIP module initialized")
end

-- Extract location data from Cloudflare headers
function _M.lookup(ip)
	-- Cloudflare provides these headers when proxying requests
	local cf_country = ngx.var.http_cf_ipcountry
	local cf_city = ngx.var.http_cf_ipcity
	local cf_region = ngx.var.http_cf_ipregion
	local cf_continent = ngx.var.http_cf_ipcontinent
	local cf_timezone = ngx.var.http_cf_timezone
	local cf_asn = ngx.var.http_cf_asn
	local cf_colo = ngx.var.http_cf_colo -- Cloudflare data center

	-- Get real IP from Cloudflare headers
	local real_ip = ngx.var.http_cf_connecting_ip or ngx.var.http_x_forwarded_for or ip

	-- Skip if no Cloudflare headers (direct access or non-CF proxy)
	if not cf_country then
		-- Fallback for direct connections or non-Cloudflare traffic
		if _M.is_private_ip(real_ip) then
			return {
				country = "private",
				country_code = "private",
				city = "private",
				source = "private_ip",
			}
		else
			return {
				country = "unknown",
				country_code = "unknown",
				city = "unknown",
				source = "no_cf_headers",
			}
		end
	end

	-- Map Cloudflare country codes to full names (partial mapping)
	local country_names = {
		US = "United States",
		CA = "Canada",
		GB = "United Kingdom",
		DE = "Germany",
		FR = "France",
		JP = "Japan",
		AU = "Australia",
		BR = "Brazil",
		IN = "India",
		CN = "China",
		RU = "Russia",
		MX = "Mexico",
		IT = "Italy",
		ES = "Spain",
		NL = "Netherlands",
		SE = "Sweden",
		NO = "Norway",
		DK = "Denmark",
		FI = "Finland",
	}

	local result = {
		country = country_names[cf_country] or cf_country or "unknown",
		country_code = cf_country or "unknown",
		city = cf_city or "unknown",
		region = cf_region or "unknown",
		continent = cf_continent or "unknown",
		timezone = cf_timezone or "unknown",
		asn = cf_asn or "unknown",
		cf_datacenter = cf_colo or "unknown",
		real_ip = real_ip,
		source = "cloudflare",
	}

	return result
end

function _M.is_private_ip(ip)
	if not ip then
		return true
	end

	local patterns = {
		"^127%.", -- 127.0.0.0/8
		"^10%.", -- 10.0.0.0/8
		"^192%.168%.", -- 192.168.0.0/16
		"^172%.1[6-9]%.", -- 172.16.0.0/12
		"^172%.2[0-9]%.", -- 172.16.0.0/12
		"^172%.3[0-1]%.", -- 172.16.0.0/12
		"^169%.254%.", -- 169.254.0.0/16 (link-local)
		"^::1$", -- IPv6 localhost
		"^fc", -- IPv6 private
		"^fd", -- IPv6 private
		"^fe80", -- IPv6 link-local
	}

	for _, pattern in ipairs(patterns) do
		if string.match(ip, pattern) then
			return true
		end
	end

	return false
end

-- Get ASN information from Cloudflare headers
function _M.get_asn_info()
	local cf_asn = ngx.var.http_cf_asn
	if cf_asn then
		return {
			asn = cf_asn,
			source = "cloudflare",
		}
	end
	return {
		asn = "unknown",
		source = "no_data",
	}
end

function _M.get_cloudflare_info()
	return {
		datacenter = ngx.var.http_cf_colo or "unknown",
		ray_id = ngx.var.http_cf_ray or "unknown",
		visitor_scheme = ngx.var.http_cf_visitor and ngx.var.http_cf_visitor:match('"scheme":"([^"]+)"') or "unknown",
		connecting_ip = ngx.var.http_cf_connecting_ip or "unknown",
	}
end

return _M
