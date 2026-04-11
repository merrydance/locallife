package api

// pageOffset calculates zero-based offset for pagination.
func pageOffset(pageID, pageSize int32) int32 {
	if pageID <= 1 || pageSize <= 0 {
		return 0
	}
	return (pageID - 1) * pageSize
}
