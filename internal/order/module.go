package order

import "net/http"

// Module is the public surface of the order vertical slice.
// main.go constructs concrete adapters and passes them here;
// everything else is an internal implementation detail of this package.
type Module struct {
	handler *Handler
}

func NewModule(write WriteRepository, read ReadRepository) *Module {
	return &Module{
		handler: NewHandler(
			NewCreateOrderHandler(write),
			NewGetOrderHandler(read),
		),
	}
}

// Register adds the module's routes to mux.
func (m *Module) Register(mux *http.ServeMux) {
	m.handler.Register(mux)
}
