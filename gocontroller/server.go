package gocontroller

import "net/http"

// Handler exposes the application router as an http.Handler.
// This makes it easy to mount in Gin/Echo/Fiber adapter wrappers.
func (a *App) Handler() http.Handler {
	return a.Router
}

func (a *App) Listen(addr string) error {
	return http.ListenAndServe(addr, a.Handler())
}
