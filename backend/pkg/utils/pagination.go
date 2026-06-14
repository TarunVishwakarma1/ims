package utils

import (
	"net/http"
	"strconv"
)

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// Pagination is parsed from query params ?page=N&page_size=M
type Pagination struct {
	Page     int
	PageSize int
}

// Offset for SQL LIMIT/OFFSET queries
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// Limit for SQL LIMIT
func (p Pagination) Limit() int {
	return p.PageSize
}

// ParsePagination reads ?page=&page_size= from request, applies defaults and clamps.
func ParsePagination(r *http.Request) Pagination {
	page := 1
	pageSize := DefaultPageSize

	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if v, err := strconv.Atoi(pStr); err == nil && v > 0 {
			page = v
		}
	}
	if sStr := r.URL.Query().Get("page_size"); sStr != "" {
		if v, err := strconv.Atoi(sStr); err == nil && v > 0 {
			if v > MaxPageSize {
				v = MaxPageSize
			}
			pageSize = v
		}
	}

	return Pagination{Page: page, PageSize: pageSize}
}

// SetPaginationHeaders writes X-Total-Count and X-Total-Pages so the
// frontend can keep consuming bare arrays (no breaking change).
func SetPaginationHeaders(w http.ResponseWriter, total int, p Pagination) {
	totalPages := total / p.PageSize
	if total%p.PageSize != 0 {
		totalPages++
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	w.Header().Set("X-Total-Pages", strconv.Itoa(totalPages))
	w.Header().Set("X-Page", strconv.Itoa(p.Page))
	w.Header().Set("X-Page-Size", strconv.Itoa(p.PageSize))
	w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count, X-Total-Pages, X-Page, X-Page-Size")
}
