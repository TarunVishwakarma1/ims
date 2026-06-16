// Package invoice renders a minimal A4 invoice PDF from an Order + its
// items. Intentionally bare-bones — header, item table, totals. Heavier
// templating (org logo, tax breakdown, custom fields) can layer later via
// an HTML/Chromium pipeline; gofpdf keeps the dependency footprint small.
package invoice

import (
	"fmt"
	"io"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/jung-kurt/gofpdf"
)

// Renderer produces invoice PDFs. Stateless.
type Renderer struct{}

func NewRenderer() *Renderer { return &Renderer{} }

// Render writes a PDF invoice for `order` + `items` to `w`. If `productNames`
// is non-nil it's used to label rows; otherwise the product UUID prefix is shown.
func (r *Renderer) Render(w io.Writer, order *domain.Order, items []*domain.OrderItem, productNames map[string]string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Header
	pdf.SetFont("Helvetica", "B", 20)
	pdf.Cell(0, 10, "INVOICE")
	pdf.Ln(12)

	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(0, 5, fmt.Sprintf("Order ID: %s", order.ID))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("Date: %s", order.CreatedAt.UTC().Format("2 Jan 2006")))
	pdf.Ln(5)
	pdf.Cell(0, 5, fmt.Sprintf("Status: %s   |   Payment: %s",
		string(order.Status), nonEmpty(order.PaymentStatus, "—")))
	pdf.Ln(10)

	if order.OrderType != "" {
		pdf.Cell(0, 5, fmt.Sprintf("Order type: %s", order.OrderType))
		pdf.Ln(5)
	}
	if order.BuyerOrgID != nil {
		pdf.Cell(0, 5, fmt.Sprintf("Buyer org: %s", *order.BuyerOrgID))
		pdf.Ln(5)
	}
	if order.SupplierOrgID != nil {
		pdf.Cell(0, 5, fmt.Sprintf("Supplier org: %s", *order.SupplierOrgID))
		pdf.Ln(5)
	}
	pdf.Ln(4)

	// Items table
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(85, 8, "Item", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, "Qty", "1", 0, "R", true, 0, "")
	pdf.CellFormat(35, 8, "Unit price", "1", 0, "R", true, 0, "")
	pdf.CellFormat(35, 8, "Amount", "1", 1, "R", true, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	for _, it := range items {
		name := productNames[it.ProductID.String()]
		if name == "" {
			name = it.ProductID.String()[:8]
		}
		pdf.CellFormat(85, 7, name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("%d", it.Quantity), "1", 0, "R", false, 0, "")
		pdf.CellFormat(35, 7, money(it.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(35, 7, money(int64(it.Quantity)*it.UnitPrice), "1", 1, "R", false, 0, "")
	}

	// Totals block — right-aligned, bottom-anchored
	pdf.Ln(4)
	totalsX := 110.0
	pdf.SetFont("Helvetica", "", 10)
	row := func(label string, amount int64) {
		pdf.SetX(totalsX)
		pdf.CellFormat(40, 6, label, "", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, money(amount), "", 1, "R", false, 0, "")
	}
	row("Subtotal", order.Subtotal)
	if order.DeliveryFee > 0 {
		row("Delivery", order.DeliveryFee)
	}
	if order.Discount > 0 {
		row("Discount", -order.Discount)
	}
	pdf.SetFont("Helvetica", "B", 11)
	row("Total", order.TotalAmount)

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(0, 5, fmt.Sprintf("Generated %s", time.Now().UTC().Format(time.RFC3339)), "", 1, "C", false, 0, "")

	return pdf.Output(w)
}

func money(paise int64) string {
	neg := ""
	if paise < 0 {
		neg = "-"
		paise = -paise
	}
	return fmt.Sprintf("%sINR %d.%02d", neg, paise/100, paise%100)
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
