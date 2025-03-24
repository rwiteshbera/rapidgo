package rapidgo

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Route struct to store route information
type Route struct {
	path    string
	method  string
	handler func(c *Context)
}

// Middleware function signature
type MiddlewareFunc func(c *Context)

// Router to manage routes and global middlewares
type Router struct {
	dynamicRoutes   map[string]*Node  // method -> tree root node
	staticRoutes    map[string]*Route // method -> path -> route
	middlewares     []MiddlewareFunc
	notFoundMessage *string
}

// RouterGroup for grouping routes with specific base paths and middleware
type RouterGroup struct {
	Router     *Router
	BasePath   string
	Middleware []MiddlewareFunc
}

// Engine is the main struct managing router groups
type Engine struct {
	Router *Router
	groups []*RouterGroup
	debug  bool
}

// New creates a new Engine instance
func New() *Engine {
	router := &Router{
		dynamicRoutes: make(map[string]*Node),
		staticRoutes:  make(map[string]*Route),
		middlewares:   []MiddlewareFunc{},
	}
	engine := &Engine{
		Router: router,
		groups: []*RouterGroup{
			{
				Router:     router,
				BasePath:   "/",
				Middleware: []MiddlewareFunc{},
			},
		},
		debug: true,
	}
	return engine
}

func (e *Engine) SetDebug(debug bool) {
	e.debug = debug
}

// Handle requests, applying middleware at the group level
func (r *RouterGroup) handle(method string, path string, handler func(c *Context)) {
	fullPath := r.BasePath + path
	if r.BasePath == "/" {
		fullPath = path
	}

	finalHandler := func(c *Context) {
		// Apply global middlewares
		for _, middleware := range r.Router.middlewares {
			c.handlers = append(c.handlers, middleware)
		}
		// Apply group-specific middlewares
		for _, middleware := range r.Middleware {
			c.handlers = append(c.handlers, middleware)
		}
		// Append the route handler
		c.handlers = append(c.handlers, handler)
		c.handlerIdx = -1
		c.Next()
	}

	r.Router.addRoute(method, fullPath, finalHandler)
}

func (r *Router) addRoute(method, path string, handler func(*Context)) {
	if IsDynamic(path) {
		if r.dynamicRoutes[method] == nil {
			r.dynamicRoutes[method] = &Node{}
		}
		r.dynamicRoutes[method].insert(path, handler)
	} else {
		routeKey := GenerateStaticRouteKey(method, path)
		r.staticRoutes[routeKey] = &Route{
			path:    path,
			method:  method,
			handler: handler,
		}
	}
}

// Find and execute the appropriate route
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	method := req.Method
	routeKey := GenerateStaticRouteKey(method, path)
	// Check if the route exists in the static routes map
	if route, exists := r.staticRoutes[routeKey]; exists {
		c := NewContext(w, req)
		route.handler(c)
		return
	} else {
		if root := r.dynamicRoutes[method]; root != nil {
			params := make(map[string]string)
			if handler := root.search(path, params); handler != nil {
				c := NewContext(w, req)
				c.params = params
				handler(c)
				return
			}
		}
	}

	if r.notFoundMessage != nil {
		http.Error(w, *r.notFoundMessage, http.StatusNotFound)
	} else {
		http.NotFound(w, req)
	}
}

// Grouping Routes
func (e *Engine) Group(path string) *RouterGroup {
	group := &RouterGroup{
		Router:     e.Router, // Use the shared router instance
		BasePath:   path,
		Middleware: []MiddlewareFunc{},
	}
	e.groups = append(e.groups, group)
	return group
}

// Use middleware at group level
func (r *RouterGroup) Use(middleware ...MiddlewareFunc) {
	r.Middleware = append(r.Middleware, middleware...)
}

// Use middleware at global level
func (e *Engine) Use(middleware ...MiddlewareFunc) {
	e.Router.middlewares = append(e.Router.middlewares, middleware...)
}

// Engine HTTP Methods
func (e *Engine) Get(path string, handler func(c *Context))     { e.groups[0].Get(path, handler) }
func (e *Engine) Post(path string, handler func(c *Context))    { e.groups[0].Post(path, handler) }
func (e *Engine) Put(path string, handler func(c *Context))     { e.groups[0].Put(path, handler) }
func (e *Engine) Delete(path string, handler func(c *Context))  { e.groups[0].Delete(path, handler) }
func (e *Engine) Patch(path string, handler func(c *Context))   { e.groups[0].Patch(path, handler) }
func (e *Engine) Options(path string, handler func(c *Context)) { e.groups[0].Options(path, handler) }
func (e *Engine) Head(path string, handler func(c *Context))    { e.groups[0].Head(path, handler) }
func (e *Engine) SetNotFoundMessage(message string)             { e.Router.notFoundMessage = &message }

// RouterGroup HTTP Methods
func (r *RouterGroup) Get(path string, handler func(c *Context))  { r.handle("GET", path, handler) }
func (r *RouterGroup) Post(path string, handler func(c *Context)) { r.handle("POST", path, handler) }
func (r *RouterGroup) Put(path string, handler func(c *Context))  { r.handle("PUT", path, handler) }
func (r *RouterGroup) Delete(path string, handler func(c *Context)) {
	r.handle("DELETE", path, handler)
}
func (r *RouterGroup) Patch(path string, handler func(c *Context)) { r.handle("PATCH", path, handler) }
func (r *RouterGroup) Options(path string, handler func(c *Context)) {
	r.handle("OPTIONS", path, handler)
}
func (r *RouterGroup) Head(path string, handler func(c *Context)) { r.handle("HEAD", path, handler) }

// Engine method to listen on a custom port
func (e *Engine) Listen(port ...string) error {
	address := ResolvePort(port)
	// if e.debug {
	// 	e.PrintRoutes(address)
	// }
	return http.ListenAndServe(address, e.Router)
}

func (e *Engine) ListenGracefully(port ...string) error {
	address := ResolvePort(port)
	srv := &http.Server{
		Addr:    address,
		Handler: e.Router,
	}

	// Start the server in a new goroutine.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Set up channel on which to send signal notifications.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit // Block until a signal is received
	log.Println("Shutdown Server ...")

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt a graceful shutdown.
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown: %s", err)
	}
	log.Println("Server exiting")
	return nil
}
