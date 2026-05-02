package order

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

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

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", h.handleCreate)
	mux.HandleFunc("GET /orders/{id}", h.handleGet)
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

	cmd, err := buildCreateOrderCmd(req)
	if err != nil {
		respondErr(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.create.Handle(r.Context(), cmd); err != nil {
		if errors.Is(err, ErrEmptyCart) || errors.Is(err, ErrInvalidQuantity) {
			respondErr(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		slog.ErrorContext(r.Context(), "create order", "order_id", cmd.OrderID, "err", err)
		respondErr(w, "internal error", http.StatusInternalServerError)
		return
	}

	respond(w, http.StatusCreated, map[string]string{"id": cmd.OrderID.String()})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
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
		slog.ErrorContext(r.Context(), "get order", "order_id", id, "err", err)
		respondErr(w, "internal error", http.StatusInternalServerError)
		return
	}

	respond(w, http.StatusOK, view)
}

// buildCreateOrderCmd is a pure function: validates and maps the HTTP request
// to a command struct with no side effects.
func buildCreateOrderCmd(req createRequest) (CreateOrderCmd, error) {
	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		return CreateOrderCmd{}, errors.New("invalid customer_id")
	}

	items := make([]Item, 0, len(req.Items))
	for _, it := range req.Items {
		pid, err := uuid.Parse(it.ProductID)
		if err != nil {
			return CreateOrderCmd{}, errors.New("invalid product_id")
		}
		items = append(items, Item{
			ProductID: pid,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
		})
	}

	return CreateOrderCmd{
		OrderID:    uuid.New(),
		CustomerID: customerID,
		Items:      items,
	}, nil
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
