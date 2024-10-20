package tcp

type Router struct {
	middlewares []func(ctx *Context)
	handlers    map[int32][]func(ctx *Context)
}

func NewRouter() *Router {
	r := &Router{
		middlewares: make([]func(ctx *Context), 0),
		handlers:    make(map[int32][]func(ctx *Context)),
	}
	return r
}

func (r *Router) Use(middleware ...func(ctx *Context)) {
	r.middlewares = append(r.middlewares, middleware...)
}

func (r *Router) Register(msgId int32, handlers ...func(ctx *Context)) {
	if _, ok := r.handlers[msgId]; ok {
		r.handlers[msgId] = append(r.handlers[msgId], handlers...)
	} else {
		r.handlers[msgId] = handlers
	}
}

func (r *Router) GetMiddlewares() []func(ctx *Context) {
	return r.middlewares
}

func (r *Router) GetHandlers(msgId int32) []func(ctx *Context) {
	return r.handlers[msgId]
}
