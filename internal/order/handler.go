package order

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler is the HTTP transport for the order vertical slice.
// It lives in the same package as the domain to avoid circular imports while
// keeping all order-related code in one place (vertical slice).
type Handler struct {
	create *CreateOrderHandler
	get    *GetOrderHandler
}

func NewHandler(create *CreateOrderHandler, get *GetOrderHandler) *Handler {
	return &Handler{create: create, get: get}
}

func (h *Handler) Mount(r chi.Router) {
	r.Post("/orders", h.handleCreate)
	r.Get("/orders/{id}", h.handleGet)
}

type createRequest struct {
	CustomerID string        `json:"customer_id"`
	Items      []itemRequest `json:"items"`
}

type itemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondErr(w, "invalid request body", http.StatusBadRequest)
		return
	}

	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		respondErr(w, "invalid customer_id", http.StatusBadRequest)
		return
	}

	items := make([]Item, 0, len(req.Items))
	for _, it := range req.Items {
		pid, err := uuid.Parse(it.ProductID)
		if err != nil {
			respondErr(w, "invalid product_id", http.StatusBadRequest)
			return
		}
		items = append(items, Item{
			ProductID: pid,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
		})
	}

	id := uuid.New()
	if err := h.create.Handle(r.Context(), CreateOrderCmd{
		OrderID:    id,
		CustomerID: customerID,
		Items:      items,
	}); err != nil {
		if errors.Is(err, ErrEmptyCart) {
			respondErr(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		respondErr(w, "internal error", http.StatusInternalServerError)
		return
	}

	respond(w, http.StatusCreated, map[string]string{"id": id.String()})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondErr(w, "invalid id", http.StatusBadRequest)
		return
	}

	view, err := h.get.Handle(r.Context(), GetOrderQuery{OrderID: id})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			respondErr(w, "not found", http.StatusNotFound)
			return
		}
		respondErr(w, "internal error", http.StatusInternalServerError)
		return
	}

	respond(w, http.StatusOK, view)
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func respondErr(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
