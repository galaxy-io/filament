package rowmodel

import "slices"

// InputFor returns the layout a resource's rows carry into a pipeline: only the
// selected columns, every column when selection is empty. A CDC delete carries
// only its key, so under cdc every non-key column is nullable.
func InputFor(source Schema, selection []string, cdc bool) Schema {
	out := source.Clone()
	if len(selection) > 0 {
		out.Fields = slices.DeleteFunc(out.Fields, func(f Field) bool { return !slices.Contains(selection, f.Name) })
	}
	if cdc {
		for i, f := range out.Fields {
			if !slices.Contains(out.PrimaryKey, f.Name) {
				out.Fields[i].Nullable = true
			}
		}
	}
	return out
}
