package gocontroller

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

// PaginationOptions holds parsed pagination parameters.
type PaginationOptions struct {
	Page     int
	Limit    int
	Offset   int
	Sort     string
	Order    string
	Total    int64
	Cursor   string
	HasMore  bool
	NextPage int
}

// PaginationConfig configures default pagination behavior.
type PaginationConfig struct {
	DefaultPage  int
	DefaultLimit int
	MaxLimit     int
}

func (c *PaginationConfig) applyDefaults() {
	if c.DefaultPage <= 0 {
		c.DefaultPage = 1
	}
	if c.DefaultLimit <= 0 {
		c.DefaultLimit = 20
	}
	if c.MaxLimit <= 0 {
		c.MaxLimit = 100
	}
}

// ParsePagination extracts pagination parameters from the request query string.
func ParsePagination(u *url.URL, cfg ...PaginationConfig) PaginationOptions {
	var c PaginationConfig
	if len(cfg) > 0 {
		c = cfg[0]
	} else {
		c = PaginationConfig{}
	}
	c.applyDefaults()

	q := u.Query()
	page := queryInt(q, "page", c.DefaultPage)
	limit := queryInt(q, "limit", c.DefaultLimit)
	cursor := q.Get("cursor")
	sort := q.Get("sort")
	order := q.Get("order")

	if limit > c.MaxLimit {
		limit = c.MaxLimit
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	}

	offset := (page - 1) * limit

	if order == "" {
		order = "asc"
	}
	order = strings.ToLower(order)
	if order != "asc" && order != "desc" {
		order = "asc"
	}

	return PaginationOptions{
		Page:   page,
		Limit:  limit,
		Offset: offset,
		Sort:   sort,
		Order:  order,
		Cursor: cursor,
	}
}

// ParsePaginationFromContext extracts pagination from the request context URL.
func ParsePaginationFromContext(ctx *Context, cfg ...PaginationConfig) PaginationOptions {
	return ParsePagination(ctx.Request.URL, cfg...)
}

// SetTotal calculates pagination metadata from total item count.
func (p *PaginationOptions) SetTotal(total int64) {
	p.Total = total
	p.HasMore = int64(p.Offset+p.Limit) < total
	if p.HasMore {
		p.NextPage = p.Page + 1
	}
}

// LinkHeader generates a RFC 5988 Link header for pagination.
func (p *PaginationOptions) LinkHeader(baseURL string) string {
	var links []string

	if p.Page > 1 {
		prevPage := p.Page - 1
		links = append(links, fmt.Sprintf("<%s?page=%d&limit=%d>; rel=\"prev\"", escapeLinkURL(baseURL), prevPage, p.Limit))
	}

	if p.HasMore {
		links = append(links, fmt.Sprintf("<%s?page=%d&limit=%d>; rel=\"next\"", escapeLinkURL(baseURL), p.NextPage, p.Limit))
	}

	totalPages := int(math.Ceil(float64(p.Total) / float64(p.Limit)))
	if totalPages > 0 {
		links = append(links, fmt.Sprintf("<%s?page=%d&limit=%d>; rel=\"last\"", escapeLinkURL(baseURL), totalPages, p.Limit))
	}

	links = append(links, fmt.Sprintf("<%s?page=1&limit=%d>; rel=\"first\"", escapeLinkURL(baseURL), p.Limit))

	return strings.Join(links, ", ")
}

// JSONResponse returns a standardized pagination JSON response body.
func (p *PaginationOptions) JSONResponse(data any) map[string]any {
	resp := map[string]any{
		"data": data,
		"pagination": map[string]any{
			"page":     p.Page,
			"limit":    p.Limit,
			"total":    p.Total,
			"has_more": p.HasMore,
		},
	}
	if p.HasMore {
		resp["pagination"].(map[string]any)["next_page"] = p.NextPage
	}
	return resp
}

func queryInt(q url.Values, key string, defaultVal int) int {
	v := q.Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func escapeLinkURL(u string) string {
	return strings.ReplaceAll(u, "&", "&amp;")
}
