package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

const defaultListLimit int32 = 25

type listPageFlags struct {
	limit int32
	next  string
}

func (f *listPageFlags) add(command *cobra.Command) {
	command.Flags().Int32Var(&f.limit, "limit", defaultListLimit, "Number of items to return")
	command.Flags().StringVar(&f.next, "next", "", "Display the next page from this cursor")
}

func (f listPageFlags) request() (model.PageRequest, error) {
	if f.limit <= 0 {
		return model.PageRequest{}, fmt.Errorf("--limit must be a positive integer")
	}
	return model.PageRequest{PageSize: f.limit, Cursor: f.next}, nil
}
