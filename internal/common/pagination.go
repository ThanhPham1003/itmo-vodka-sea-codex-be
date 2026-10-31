package common

import (
	"fmt"
	"strconv"
)

type Page struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func ParsePage(limit, offset string) (Page, error) {
	p := Page{Limit: 20}
	var err error
	if limit != "" {
		p.Limit, err = strconv.Atoi(limit)
		if err != nil {
			return p, fmt.Errorf("%w: invalid limit", ErrInvalidArgument)
		}
	}
	if offset != "" {
		p.Offset, err = strconv.Atoi(offset)
		if err != nil {
			return p, fmt.Errorf("%w: invalid offset", ErrInvalidArgument)
		}
	}
	if p.Limit < 1 || p.Limit > 100 || p.Offset < 0 {
		return p, fmt.Errorf("%w: invalid pagination", ErrInvalidArgument)
	}
	return p, nil
}

type List[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page
}
