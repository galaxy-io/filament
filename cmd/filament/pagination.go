package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

const defaultListLimit int32 = 25

// listPageFlags are the paging flags every list command shares.
type listPageFlags struct {
	limit int32
	next  string
}

func (f *listPageFlags) add(command *cobra.Command) {
	command.Flags().Int32Var(&f.limit, "limit", defaultListLimit, "Number of items to show")
	command.Flags().StringVar(&f.next, "next", "", "Show the page after this `CURSOR`")
}

func (f listPageFlags) request() (model.PageRequest, error) {
	if f.limit <= 0 {
		return model.PageRequest{}, fmt.Errorf("--limit must be a positive integer")
	}
	return model.PageRequest{PageSize: f.limit, Cursor: f.next}, nil
}

// pageRequestFromFlags reads --limit and --next out of a dynamic command's
// parsed flags.
func pageRequestFromFlags(flags map[string][]string) (model.PageRequest, error) {
	page := listPageFlags{limit: defaultListLimit, next: lastFlag(flags, "next")}
	if raw, present := flagValue(flags, "limit"); present {
		if _, err := fmt.Sscanf(raw, "%d", &page.limit); err != nil {
			return model.PageRequest{}, fmt.Errorf("--limit must be a positive integer")
		}
	}
	return page.request()
}
