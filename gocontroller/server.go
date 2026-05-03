package gocontroller

import "net/http"

// Handler exposes the application router as an http.Handler.
// This makes it easy to mount in Gin/Echo/Fiber adapter wrappers.
func (a *App) Handler() http.Handler {
	return a.Router
}

func (a *App) Listen(addr string) error {
	return a.NewHTTPServer(ServerOptions{Addr: addr}).ListenAndServe()
}

func (a *App) ListenTLS(addr, certFile, keyFile string) error {
	return a.NewHTTPServer(ServerOptions{Addr: addr}).ListenAndServeTLS(certFile, keyFile)
}
