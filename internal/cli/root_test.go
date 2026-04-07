package cli

import "github.com/pbsladek/knotical/internal/app"

func rootReq(configure func(*app.Request)) app.Request {
	var req app.Request
	if configure != nil {
		configure(&req)
	}
	return req
}
