package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"
)

// BuildReceiptPDF renders the same receipt + tax breakdown as the in-app
// receipt page into a single-page A4 PDF. Uses gofpdf (pure Go, no external
// runtime dependencies). Returns the PDF bytes, or an error if data lookup
// fails.
func (s *paymentService) BuildReceiptPDF(ctx context.Context, paymentID, orgID uuid.UUID) ([]byte, error) {
	receipt, err := s.BuildReceipt(ctx, paymentID, orgID)
	if err != nil {
		return nil, err
	}
	payment, err := s.paymentRepo.GetByID(ctx, paymentID, orgID)
	if err != nil {
		return nil, err
	}
	if payment.OrderID == nil {
		return nil, errors.New("payment has no linked order")
	}
	order, err := s.orderRepo.GetByID(ctx, *payment.OrderID, orgID)
	if err != nil {
		return nil, err
	}
	refunds, _ := s.paymentRepo.ListRefunds(ctx, paymentID, orgID)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Header band
	pdf.SetFillColor(79, 70, 229) // indigo-600
	pdf.Rect(0, 0, 210, 38, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetY(12)
	pdf.SetX(15)
	pdf.CellFormat(100, 8, "Tax Invoice", "", 0, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetY(13)
	pdf.SetX(115)
	pdf.CellFormat(80, 6, receipt.OrgName, "", 0, "R", false, 0, "")
	if receipt.InvoiceNumber != "" {
		pdf.SetY(20)
		pdf.SetX(115)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(80, 6, receipt.InvoiceNumber, "", 0, "R", false, 0, "")
	}

	pdf.SetY(45)
	pdf.SetTextColor(15, 23, 42)

	// Buyer + dates block
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(100, 116, 139)
	pdf.CellFormat(95, 5, "BILLED TO", "", 0, "L", false, 0, "")
	pdf.CellFormat(80, 5, "DETAILS", "", 1, "L", false, 0, "")

	pdf.SetTextColor(15, 23, 42)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(95, 6, receipt.BuyerName, "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(35, 6, "Captured:", "", 0, "L", false, 0, "")
	pdf.CellFormat(45, 6, receipt.CapturedAt.Format("02 Jan 2006, 15:04"), "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(95, 5, receipt.ToEmail, "", 0, "L", false, 0, "")
	pdf.CellFormat(35, 5, "Order ID:", "", 0, "L", false, 0, "")
	pdf.CellFormat(45, 5, pdfShortID(receipt.OrderID), "", 1, "L", false, 0, "")

	pdf.CellFormat(95, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(35, 5, "Payment ID:", "", 0, "L", false, 0, "")
	pdf.CellFormat(45, 5, pdfShortID(receipt.PaymentID), "", 1, "L", false, 0, "")

	if receipt.RazorpayPayment != "" {
		pdf.CellFormat(95, 5, "", "", 0, "L", false, 0, "")
		pdf.CellFormat(35, 5, "Razorpay ref:", "", 0, "L", false, 0, "")
		pdf.CellFormat(45, 5, receipt.RazorpayPayment, "", 1, "L", false, 0, "")
	}

	pdf.Ln(4)

	// Items table
	pdf.SetFillColor(243, 244, 246)
	pdf.SetTextColor(71, 85, 105)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(105, 8, "  Item", "", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, "Qty", "", 0, "R", true, 0, "")
	pdf.CellFormat(50, 8, "Amount  ", "", 1, "R", true, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(15, 23, 42)
	for _, line := range receipt.Items {
		pdf.CellFormat(105, 7, "  "+line.Name, "B", 0, "L", false, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("%d", line.Qty), "B", 0, "R", false, 0, "")
		pdf.CellFormat(50, 7, line.Total+"  ", "B", 1, "R", false, 0, "")
	}

	pdf.Ln(2)

	// Totals block — right-aligned
	totalsX := 105.0
	row := func(label string, value int64, bold bool) {
		pdf.SetX(15 + totalsX)
		if bold {
			pdf.SetFont("Helvetica", "B", 10)
		} else {
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(71, 85, 105)
		}
		pdf.CellFormat(40, 5, label, "", 0, "L", false, 0, "")
		pdf.SetTextColor(15, 23, 42)
		if bold {
			pdf.SetFont("Helvetica", "B", 10)
		} else {
			pdf.SetFont("Helvetica", "", 10)
		}
		pdf.CellFormat(45, 5, pdfMoney(value), "", 1, "R", false, 0, "")
	}
	rowText := func(label, value string, bold bool) {
		pdf.SetX(15 + totalsX)
		if bold {
			pdf.SetFont("Helvetica", "B", 10)
		} else {
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(71, 85, 105)
		}
		pdf.CellFormat(40, 5, label, "", 0, "L", false, 0, "")
		pdf.SetTextColor(15, 23, 42)
		if bold {
			pdf.SetFont("Helvetica", "B", 10)
		} else {
			pdf.SetFont("Helvetica", "", 10)
		}
		pdf.CellFormat(45, 5, value, "", 1, "R", false, 0, "")
	}

	row("Subtotal", order.Subtotal, false)
	if order.Discount > 0 {
		rowText("Discount", "-"+pdfMoney(order.Discount), false)
	}
	if order.IsInterState {
		if order.TaxIGST > 0 {
			row("IGST", order.TaxIGST, false)
		}
	} else {
		if order.TaxCGST > 0 {
			row("CGST", order.TaxCGST, false)
		}
		if order.TaxSGST > 0 {
			row("SGST", order.TaxSGST, false)
		}
	}
	if order.DeliveryFee > 0 {
		row("Delivery", order.DeliveryFee, false)
	}

	// Total
	pdf.SetX(15 + totalsX)
	pdf.SetDrawColor(203, 213, 225)
	pdf.Line(15+totalsX, pdf.GetY(), 15+totalsX+85, pdf.GetY())
	pdf.Ln(1)
	row("Total paid", receipt.AmountPaise, true)

	if payment.AmountRefunded > 0 {
		rowText("Refunded", "-"+pdfMoney(payment.AmountRefunded), false)
		row("Net paid", receipt.AmountPaise-payment.AmountRefunded, true)
	}

	// Refund history section
	if len(refunds) > 0 {
		pdf.Ln(6)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetTextColor(71, 85, 105)
		pdf.CellFormat(180, 6, "REFUNDS ISSUED", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(15, 23, 42)
		for _, r := range refunds {
			ref := "—"
			if r.RazorpayRefundID != nil {
				ref = *r.RazorpayRefundID
			}
			pdf.CellFormat(85, 6, ref, "B", 0, "L", false, 0, "")
			pdf.CellFormat(50, 6, r.CreatedAt.Format("02 Jan 2006"), "B", 0, "L", false, 0, "")
			pdf.CellFormat(45, 6, "-"+pdfMoney(r.Amount), "B", 1, "R", false, 0, "")
		}
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(100, 116, 139)
	pdf.MultiCell(180, 4,
		"This is a computer-generated tax invoice. For refunds or queries, contact "+receipt.OrgName+" with the payment ID above.",
		"", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// pdfShortID renders just the first uuid block for compact display.
func pdfShortID(id uuid.UUID) string {
	s := id.String()
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}

// pdfMoney formats paise as "Rs. 12345.67". gofpdf's default Helvetica core
// font lacks the Unicode rupee glyph, so we use the ASCII fallback.
func pdfMoney(paise int64) string {
	return fmt.Sprintf("Rs. %.2f", float64(paise)/100.0)
}
