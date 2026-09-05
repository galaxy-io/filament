package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// listPageFlags are the paging flags every list command shares.
type listPageFlags struct {
	limit int32
	next  string
}

func (f *listPageFlags) add(command *cobra.Command) {
	command.Flags().Int32Var(&f.limit, "limit", model.DefaultPageSize, "Number of items to show")
	command.Flags().StringVar(&f.next, "next", "", "Show the page after this `CURSOR`")
}

func (f listPageFlags) request() (model.PageRequest, error) {
	if f.limit <= 0 {
		return model.PageRequest{}, fmt.Errorf("--limit must be a positive integer")
	}
	return model.PageRequest{PageSize: f.limit, Cursor: f.next}, nil
}
