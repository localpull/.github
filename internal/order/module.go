package order

import "github.com/go-chi/chi/v5"

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

// Mount registers the module's HTTP routes on the provided router.
func (m *Module) Mount(r chi.Router) {
	m.handler.Mount(r)
}
