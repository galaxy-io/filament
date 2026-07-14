package manifest

// SortResources returns resources in dependency order — parents before children.
// Safe with a subset; references to resources outside the slice are treated as
// already-resolved externals.
func SortResources(resources []Resource) []Resource {
	byName := make(map[string]int, len(resources))
	for i, r := range resources {
		byName[r.Name] = i
	}
	visited := make(map[string]bool, len(resources))
	sorted := make([]Resource, 0, len(resources))

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		idx, exists := byName[name]
		if !exists {
			visited[name] = true
			return
		}
		visited[name] = true
		r := resources[idx]
		if r.Parent != nil {
			visit(r.Parent.Resource)
		}
		sorted = append(sorted, r)
	}
	for _, r := range resources {
		visit(r.Name)
	}
	return sorted
}

// Split returns top-level resources and child resources separately.
// Child resources are those with a non-nil Parent reference.
func Split(resources []Resource) (topLevel, children []Resource) {
	for _, r := range resources {
		if r.Parent != nil {
			children = append(children, r)
		} else {
			topLevel = append(topLevel, r)
		}
	}
	return topLevel, children
}
