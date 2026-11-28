package server

import (
	"testing"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func TestListOptionsOfUsesTypedSorting(t *testing.T) {
	options, err := listOptionsOf(
		&ingestionv1.PaginationRequest{PageSize: 10},
		"  orders  ",
		&ingestionv1.SortingRequest{
			SortBy:    ingestionv1.SortBy_SORT_BY_NAME,
			SortOrder: ingestionv1.SortOrder_SORT_ORDER_ASC,
		},
		map[ingestionv1.SortBy]string{ingestionv1.SortBy_SORT_BY_NAME: "name"},
		"id",
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if options.Search != "orders" || options.SortBy != "name" || options.SortDescending || options.Limit != 10 {
		t.Fatalf("options = %+v", options)
	}
}

func TestListOptionsOfRejectsUnsupportedTypedSort(t *testing.T) {
	_, err := listOptionsOf(
		nil,
		"",
		&ingestionv1.SortingRequest{SortBy: ingestionv1.SortBy_SORT_BY_NAME},
		map[ingestionv1.SortBy]string{ingestionv1.SortBy_SORT_BY_CREATED_AT: "created_at"},
		"version",
		true,
	)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("error = %v, want InvalidArgument", err)
	}
}

func TestListOptionsOfPreservesDefaultDirection(t *testing.T) {
	for _, descending := range []bool{false, true} {
		for _, sorting := range []*ingestionv1.SortingRequest{
			nil,
			{},
			{SortBy: ingestionv1.SortBy_SORT_BY_NAME},
		} {
			options, err := listOptionsOf(nil, "", sorting,
				map[ingestionv1.SortBy]string{ingestionv1.SortBy_SORT_BY_NAME: "name"}, "id", descending)
			if err != nil {
				t.Fatal(err)
			}
			if options.SortDescending != descending {
				t.Errorf("sorting %v with default descending %v: got %v", sorting, descending, options.SortDescending)
			}
		}
	}
}

func TestListOptionsOfExplicitDirectionOverridesDefault(t *testing.T) {
	for _, descending := range []bool{false, true} {
		for _, order := range []ingestionv1.SortOrder{
			ingestionv1.SortOrder_SORT_ORDER_ASC,
			ingestionv1.SortOrder_SORT_ORDER_DESC,
		} {
			options, err := listOptionsOf(nil, "", &ingestionv1.SortingRequest{SortOrder: order}, nil, "id", descending)
			if err != nil {
				t.Fatal(err)
			}
			if want := order == ingestionv1.SortOrder_SORT_ORDER_DESC; options.SortDescending != want {
				t.Errorf("order %v with default descending %v: got %v", order, descending, options.SortDescending)
			}
		}
	}
}
