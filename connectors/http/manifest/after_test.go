package manifest

import (
	"strings"
	"testing"
)

const afterBase = `
version: 1
name: ordered
display_name: Ordered
description: Read order fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
resources:
  - name: invoices
    path: /invoices
    capture: {invoice_id: id}
%s
  - name: invoice_items
    path: /invoice-items
%s
  - name: invoice_notes
    path: /invoices/{invoice_id}/notes
    for_each: invoices
    params: {invoice_id: parent.invoice_id}
`

func parseAfter(t *testing.T, invoices, items string) error {
	t.Helper()
	_, err := Parse([]byte(strings.Replace(strings.Replace(afterBase, "%s", invoices, 1), "%s", items, 1)))
	return err
}

func TestAfterAcceptsTopLevelOrder(t *testing.T) {
	m, err := Parse([]byte(strings.Replace(strings.Replace(afterBase, "%s", "    after: [invoice_items]", 1), "%s", "", 1)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := m.Resources[0].After; len(got) != 1 || got[0] != "invoice_items" {
		t.Fatalf("after = %v", got)
	}
}

func TestAfterRejectsInvalidOrder(t *testing.T) {
	for _, tc := range []struct {
		name, invoices, items, want string
	}{
		{"unknown", "    after: [missing]", "", `unknown resource "missing"`},
		{"self", "    after: [invoices]", "", "cannot wait on itself"},
		{"child dependency", "    after: [invoice_notes]", "", "is a child"},
		{"cycle", "    after: [invoice_items]", "    after: [invoices]", "after cycle"},
		{"duplicate", "    after: [invoice_items, invoice_items]", "", "after"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := parseAfter(t, tc.invoices, tc.items)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestAfterRejectsChildResource(t *testing.T) {
	data := strings.Replace(strings.Replace(afterBase, "%s", "", 1), "%s", "", 1)
	data = strings.Replace(data, "    for_each: invoices\n", "    for_each: invoices\n    after: [invoice_items]\n", 1)
	if _, err := Parse([]byte(data)); err == nil || !strings.Contains(err.Error(), "only supported on top-level") {
		t.Fatalf("err = %v", err)
	}
}
