package main

import (
	"bytes"
	"testing"
)

func TestRenderConsignmentPDFIncludesAllCopiesAndTemplateArtwork(t *testing.T) {
	record := consignment{
		Transporter:       transporter{Name: "Test Transport"},
		ConsignmentNumber: "TEST-8468",
		Date:              "2026-11-23",
		InsuranceStatus:   "not_insured",
		Consignor:         party{Name: "Sender"},
		Consignee:         party{Name: "Receiver"},
		Origin:            "Pune",
		Destination:       "Surat",
		Items:             []consignmentItem{{Description: "Industrial Air Filter"}},
	}

	output, err := renderConsignmentPDF(record)
	if err != nil {
		t.Fatalf("render PDF: %v", err)
	}
	if !bytes.HasPrefix(output, []byte("%PDF-")) {
		t.Fatal("output does not have a PDF header")
	}
	if !bytes.Contains(output, []byte("/Subtype /Image")) {
		t.Fatal("output does not include the original template artwork")
	}
	pageCount := bytes.Count(output, []byte("/Type /Page")) - bytes.Count(output, []byte("/Type /Pages"))
	if pageCount != 3 {
		t.Fatalf("expected all three copy pages, got %d", pageCount)
	}
}
