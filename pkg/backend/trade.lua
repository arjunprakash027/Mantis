local portfolio_key = KEYS[1]
local trade_log_key = KEYS[2]

local action = ARGV[1]
local asset = ARGV[2]
local amount = tonumber(ARGV[3])
local price = tonumber(ARGV[4])
local total_cost = tonumber(ARGV[5]) 
local timestamp = ARGV[6]
local strategy_id = ARGV[7]

local meta_key = "token:meta:" .. asset
local outcome = redis.call('HGET', meta_key, 'outcome') or 'unknown'
local market = redis.call('HGET', meta_key, 'market') or 'unknown'

if action == "BUY" then
    local usd_balance = tonumber(redis.call('HGET', portfolio_key, 'USD') or 0)
    if usd_balance < total_cost then
        return {0, "Insufficient USD funds"}
    end
    redis.call('HINCRBYFLOAT', portfolio_key, 'USD', -total_cost)
    redis.call('HINCRBYFLOAT', portfolio_key, asset, amount)
    
elseif action == "SELL" then
    local asset_balance = tonumber(redis.call('HGET', portfolio_key, asset) or 0)
    if asset_balance < amount then
        return {0, "Insufficient asset balance"}
    end
    redis.call('HINCRBYFLOAT', portfolio_key, asset, -amount)
    redis.call('HINCRBYFLOAT', portfolio_key, 'USD', total_cost)
end

local final_usd = redis.call('HGET', portfolio_key, 'USD')
local final_asset = redis.call('HGET', portfolio_key, asset)

redis.call('XADD', trade_log_key, '*', 
    'action', action, 'asset_id', asset, 'market', market, 'outcome', outcome,
    'amount', amount, 'price', price, 'total', total_cost, 
    'balance_usd', final_usd, 'balance_asset', final_asset,
    'strategy', strategy_id, 'timestamp', timestamp
)

return {1, "Success"}
