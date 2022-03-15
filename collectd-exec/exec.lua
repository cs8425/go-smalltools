-- render collectd-exec data
-- put in: /usr/lib/lua/luci/statistics/rrdtool/definitions/
-- clear luci's cache with: rm -rf /tmp/luci-indexcache /tmp/luci-modulecache/

module("luci.statistics.rrdtool.definitions.exec", package.seeall)

function item()
	return luci.i18n.translate("Exec")
end

function rrdargs(graph, plugin, plugin_instance, dtype)
	-- For $HOSTNAME/exec-foo-bar/temperature_baz-quux.rrd, plugin will be
	-- "exec" and plugin_instance will be "foo-bar".  I guess graph will be "baz-quux"?  We may also be ignoring a fourth argument, dtype.
	if "cpufreq" == plugin_instance then
		return {
			title = "%H: CPU frequency #%pi",
			vlabel = "MHz",
			number_format = "%5.0lf MHz",
			-- number_format = "%1.2lf GHz",
			data = {
				-- types = { "frequency" },
				-- types = { "temperature", "cpufreq", "frequency" }, 
				-- options = {
				-- 	frequency = {
				-- 		-- Convert to farenheit. See /usr/lib/lua/luci/statistics/rrdtool.lua and rrdgraph_rpn manpage.
				-- 		-- transform_rpn = "1.8,*,32,+",
				-- 		title  = "made up",
				-- 		color  = "ff0000"
				-- 	}
				-- }

				-- data type order
				types = { "frequency" },

				-- defined sources for data types, if ommitted assume a single DS named "value" (optional)
				sources = {
					frequency = {
						"value",
					},
				},

				-- special options for single data lines
				options = {
					frequency__value = {
						noarea = true,
						overlay = true, -- don't summarize
						-- color   = "00ff00",
						title = "%di",
					}
				}
			}
		}
	end

	local inst = plugin_instance:gsub("^request-%(.+)$", "%1")
	if #inst > 1 then

		return {
			title = "%%H: HTTP request \"%s\"" % inst,
			vlabel = "ms",
			number_format = "%5.1lf ms",
			data = {
				-- data type order
				types = { "latency" },

				-- defined instances (rrd file)
				instances = {
					latency = {
						"Connect",
						"TTFB",
						"DNS",
						"total",
					},
				},

				-- special options for single data lines
				options = {
					latency_Connect = {
						-- Convert to farenheit. See /usr/lib/lua/luci/statistics/rrdtool.lua and rrdgraph_rpn manpage.
						-- transform_rpn = "1.8,*,32,+",
						noarea = true,
						overlay = true, -- don't summarize
						color   = "00ff00",
						title = "%di",
					},
					latency_TTFB = {
						noarea = true,
						overlay = true, -- don't summarize
						color   = "0000ff",
						title = "%di",
					},
					latency_DNS = {
						noarea = true,
						overlay = true, -- don't summarize
						color   = "ff5500",
						title = "%di",
					},
					latency_total = {
						noarea = true,
						overlay = true, -- don't summarize
						color   = "ff00ff",
						title = "%di",
					},
				}
			}
		}
	end
end
