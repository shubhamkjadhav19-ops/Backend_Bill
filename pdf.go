package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

//go:embed templates/consignment-note.png
var consignmentNoteTemplate []byte

const (
	templateHeightPoints = 841.89
	pointsPerMillimeter  = 72.0 / 25.4
)

func renderConsignmentPDF(record consignment) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(fmt.Sprintf("Consignment Note %s", pdfText(record.ConsignmentNumber)), true)
	pdf.SetAuthor(pdfText(record.Transporter.Name), true)

	options := gofpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("consignment-template", options, bytes.NewReader(consignmentNoteTemplate))
	for _, copyType := range []string{"Consignor copy", "Consignee copy", "Driver copy"} {
		pdf.AddPage()
		pdf.ImageOptions("consignment-template", 0, 0, 210, 297, false, options, 0, "")
		drawTemplateValues(pdf, record, copyType)
	}

	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, err
	}
	if err := pdf.Error(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func pdfText(value string) string {
	replacer := strings.NewReplacer(
		"₹", "Rs. ",
		"–", "-",
		"—", "-",
		"’", "'",
		"“", "\"",
		"”", "\"",
		"•", "-",
	)
	return replacer.Replace(value)
}

func drawTemplateValues(pdf *gofpdf.Fpdf, record consignment, copyType string) {
	canvas := templateCanvas{pdf: pdf}

	canvas.replaceColor(168, 777, 260, 21, 21, pdfText(record.Transporter.Name), "C", "B", 31, 111, 176)
	canvas.replace(150, 759.3, 300, 9, 9, pdfText(record.Transporter.Address), "C", "")
	canvas.replace(254.7, 745, 120, 9, 9, " "+pdfText(record.Transporter.Email), "L", "")
	canvas.replace(518.6, 802, 55, 9, 9, pdfText(record.Transporter.Phone), "R", "")
	canvas.replace(521.1, 788.5, 52, 9, 9, pdfText(record.Transporter.AlternatePhone), "R", "")
	canvas.replace(230, 802, 130, 9, 9, "Subject to "+pdfText(record.Transporter.Jurisdiction)+" Jurisdiction", "C", "")
	canvas.replaceColor(210, 711.3, 170, 9, 9, "ENROLMENT NO "+pdfText(record.Transporter.EnrollmentNumber), "C", "B", 31, 111, 176)

	canvas.replaceColor(70, 599.7, 85, 13.5, 13.5, "No - "+pdfText(record.ConsignmentNumber)+".", "C", "", 31, 111, 176)
	canvas.replace(60, 576.7, 110, 9, 9, "Date - "+formatTemplateDate(record.Date), "C", "")

	copyLabel := strings.ToUpper(strings.TrimSpace(copyType))
	canvas.replaceColor(245, 693.3, 105, 9, 9, copyLabel, "C", "B", 31, 111, 176)

	payer := strings.ToUpper(pdfText(record.GSTPayableBy))
	canvas.replace(453, 696.3, 70, 9, 9, payer, "C", "")
	canvas.replace(443.5, 572, 120, 9, 9, pdfText(record.VehicleNumber), "L", "")
	canvas.replace(428.5, 549.5, 130, 9, 9, pdfText(record.Origin), "L", "")
	canvas.replace(417, 527, 130, 9, 9, pdfText(record.Destination), "L", "")
	canvas.replace(467, 504.5, 100, 9, 9, pdfText(record.EWayBillNumber), "L", "")

	canvas.replace(89.3, 547.4, 275, 8.3, 8.3, pdfText(record.Consignor.Name), "L", "")
	canvas.replaceLines(28, []float64{533.9, 520.4}, 337, 8.3, 10, record.Consignor.Address)
	canvas.replace(70.5, 506.9, 147, 8.3, 8.3, pdfText(record.Consignor.GSTNumber), "L", "")

	canvas.replace(85.7, 489.7, 276, 8.3, 8.3, pdfText(record.Consignee.Name), "L", "")
	canvas.replaceLines(28, []float64{475.5, 461.2}, 337, 8.3, 10, record.Consignee.Address)
	canvas.replace(68.3, 447, 150, 8.3, 8.3, pdfText(record.Consignee.GSTNumber), "L", "")

	if record.InsuranceStatus == "insured" {
		canvas.replace(210, 603.4, 175, 7.5, 7.5, "Company "+pdfText(record.InsuranceCompany), "L", "")
		canvas.replace(210, 590.8, 84, 7.5, 7.5, "Policy No : "+pdfText(record.InsurancePolicyNumber), "L", "")
		canvas.replace(292, 590.8, 90, 7.5, 7.5, "Date : "+formatTemplateDate(record.InsuranceDate), "L", "")
		canvas.replace(210, 578.2, 84, 7.5, 7.5, "Amount : "+pdfText(record.InsuranceAmount), "L", "")
		canvas.replace(292, 578.2, 90, 7.5, 7.5, "Risk : Owner Risk", "L", "")
		mark := func(x, y float64) {
			canvas.text(x, y, "X", 7.5, "B")
		}
		mark(363, 628)
	} else {
		canvas.replace(210, 603.4, 175, 7.5, 7.5, "Company ............................................................", "L", "")
		canvas.replace(210, 590.8, 84, 7.5, 7.5, "Policy No : .................", "L", "")
		canvas.replace(292, 590.8, 90, 7.5, 7.5, "Date : ......................", "L", "")
		canvas.replace(210, 578.2, 84, 7.5, 7.5, "Amount : ..................", "L", "")
		canvas.replace(292, 578.2, 90, 7.5, 7.5, "Risk : Owner Risk", "L", "")
		canvas.replace(210, 628, 175, 7.5, 7.5, "He has insured the Consignemnt OR", "L", "")
		canvas.replace(210, 619, 175, 7.5, 7.5, "He has not insured the Consignemnt", "L", "")
		canvas.text(365, 619, "X", 7.5, "B")
	}

	drawTemplateItems(canvas, record)
	canvas.replace(129, 261.6, 100, 7.5, 7.5, pdfText(record.DeclaredValue), "L", "")
	canvas.replace(293.6, 261.6, 75, 7.5, 7.5, pdfText(record.VehicleType), "L", "")
	canvas.replace(435, 245.3, 90, 7.5, 7.5, pdfText(record.Transporter.Name), "C", "B")

}

func drawTemplateItems(canvas templateCanvas, record consignment) {
	if len(record.Items) == 0 {
		return
	}
	quantity := itemValues(record.Items, func(item consignmentItem) string { return item.Quantity })
	weight := itemValues(record.Items, func(item consignmentItem) string { return item.Weight })
	description := itemValues(record.Items, func(item consignmentItem) string { return item.Description })
	invoice := itemValues(record.Items, func(item consignmentItem) string { return item.InvoiceNumber })
	rates := itemValues(record.Items, func(item consignmentItem) string { return item.Rate })
	amounts := itemValues(record.Items, func(item consignmentItem) string { return item.Amount })
	payments := itemValues(record.Items, func(item consignmentItem) string { return item.Payment })
	freight := "Freight"
	if rates != "" {
		freight += ": " + rates
	}

	canvas.replace(32.7, 392.5, 34, 7.5, 7.5, quantity, "L", "")
	canvas.replace(70, 392.5, 157, 7.5, 7.5, description, "C", "")
	canvas.replace(228, 392.5, 50, 7.5, 7.5, weight, "C", "")
	canvas.replace(280, 392.5, 60, 7.5, 7.5, freight, "C", "")
	canvas.replace(364, 392.5, 42, 7.5, 7.5, amounts, "R", "")
	canvas.replace(407, 392.5, 47, 7.5, 7.5, payments, "C", "")
	canvas.replace(455, 392.5, 114, 7.5, 7.5, invoice, "C", "")

	charges := []struct {
		value string
		y     float64
	}{
		{record.Charges.RTO, 376.1},
		{record.Charges.AdditionalOtherCharges, 359.8},
		{record.Charges.Hamali, 343.4},
		{record.Charges.Other, 327.1},
		{record.Charges.Door, 310.7},
		{record.Charges.Station, 294.4},
		{record.Total, 278},
	}
	for _, row := range charges {
		if row.value == "" {
			continue
		}
		canvas.replace(364, row.y, 42, 7.5, 7.5, pdfText(row.value), "R", "")
	}
}

func itemValues(items []consignmentItem, selectValue func(consignmentItem) string) string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(pdfText(selectValue(item)))
		if value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, " / ")
}

type templateCanvas struct {
	pdf *gofpdf.Fpdf
}

func (canvas templateCanvas) replace(x, baseline, width, fontSize, maskHeight float64, text, alignment, style string) {
	canvas.replaceColor(x, baseline, width, fontSize, maskHeight, text, alignment, style, 0, 0, 0)
}

func (canvas templateCanvas) replaceColor(x, baseline, width, fontSize, maskHeight float64, text, alignment, style string, red, green, blue int) {
	canvas.mask(x, baseline, width, maskHeight)
	if strings.TrimSpace(text) == "" {
		return
	}
	canvas.pdf.SetTextColor(red, green, blue)
	canvas.pdf.SetFont("Arial", style, fontSize)
	for canvas.pdf.GetStringWidth(text) > width/pointsPerMillimeter && fontSize > 5 {
		fontSize -= .5
		canvas.pdf.SetFont("Arial", style, fontSize)
	}
	canvas.pdf.SetFont("Arial", style, fontSize)
	xMM := x / pointsPerMillimeter
	yMM := (templateHeightPoints - baseline) / pointsPerMillimeter
	widthMM := width / pointsPerMillimeter
	switch alignment {
	case "C":
		textWidth := canvas.pdf.GetStringWidth(text)
		xMM += math.Max(0, (widthMM-textWidth)/2)
	case "R":
		textWidth := canvas.pdf.GetStringWidth(text)
		xMM += math.Max(0, widthMM-textWidth)
	}
	canvas.pdf.Text(xMM, yMM, text)
}

func (canvas templateCanvas) replaceLines(x float64, baselines []float64, width, fontSize, maskHeight float64, text string) {
	size := fontSize
	lines := []string(nil)
	for {
		canvas.pdf.SetFont("Arial", "", size)
		lines = wrapPDFText(canvas.pdf, pdfText(text), width/pointsPerMillimeter)
		if len(lines) <= len(baselines) || size <= 5 {
			break
		}
		size -= .5
	}
	for index, baseline := range baselines {
		canvas.mask(x, baseline, width, maskHeight)
		if index < len(lines) {
			canvas.pdf.SetTextColor(0, 0, 0)
			canvas.pdf.SetFont("Arial", "", size)
			canvas.pdf.Text(x/pointsPerMillimeter, (templateHeightPoints-baseline)/pointsPerMillimeter, lines[index])
		}
	}
}

func (canvas templateCanvas) text(x, baseline float64, text string, fontSize float64, style string) {
	canvas.pdf.SetTextColor(0, 0, 0)
	canvas.pdf.SetFont("Arial", style, fontSize)
	canvas.pdf.Text(x/pointsPerMillimeter, (templateHeightPoints-baseline)/pointsPerMillimeter, pdfText(text))
}

func wrapPDFText(pdf *gofpdf.Fpdf, text string, width float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		candidate := line + " " + word
		if pdf.GetStringWidth(candidate) > width {
			lines = append(lines, line)
			line = word
		} else {
			line = candidate
		}
	}
	return append(lines, line)
}

func (canvas templateCanvas) mask(x, baseline, width, fontSize float64) {
	top := templateHeightPoints - (baseline + fontSize*0.8)
	height := fontSize * 1.25
	canvas.pdf.SetFillColor(255, 255, 255)
	canvas.pdf.Rect(x/pointsPerMillimeter, top/pointsPerMillimeter, width/pointsPerMillimeter, height/pointsPerMillimeter, "F")
}

func formatTemplateDate(value string) string {
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.Format("02-Jan-2006")
	}
	return pdfText(value)
}
