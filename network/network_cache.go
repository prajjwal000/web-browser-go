package network

import "time"

type ResponseCache struct {
	Cache      map[string]Response
	TimeToLive map[string]int64 // in seconds
}

func (req *Request) CacheResponse(resp Response, ttl int64) {
	if req.ResponseCache.Cache == nil {
		req.ResponseCache.Cache = make(map[string]Response)
		req.ResponseCache.TimeToLive = make(map[string]int64)
	}

	req.ResponseCache.Cache[req.Path] = resp
	req.ResponseCache.TimeToLive[req.Path] = ttl + time.Now().Unix()
}
