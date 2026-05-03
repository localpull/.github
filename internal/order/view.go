package order

import "time"

// OrderView is the read-model DTO returned to callers.
// Intentionally separate from the Order aggregate so the two can evolve
// independently (e.g. add denormalised fields without touching domain).
type OrderView struct {
	ID         string     `json:"id"`
	CustomerID string     `json:"customer_id"`
	Status     string     `json:"status"`
	Items      []ItemView `json:"items"`
	CreatedAt  string     `json:"created_at"`
}

type ItemView struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
}

func toView(o Order) OrderView {
	items := make([]ItemView, len(o.Items))
	for i, it := range o.Items {
		items[i] = ItemView{
			ProductID: it.ProductID.String(),
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
		}
	}
	return OrderView{
		ID:         o.ID.String(),
		CustomerID: o.CustomerID.String(),
		Status:     string(o.Status),
		Items:      items,
		CreatedAt:  o.CreatedAt.Format(time.RFC3339),
	}
}
