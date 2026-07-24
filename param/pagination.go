package param

const (
	DefaultPageNumber = 1
	DefaultPageSize   = 25
)

type PaginationRequest struct {
	PageSize   int `json:"page_size" form:"page_size" query:"page_size"`
	PageNumber int `json:"page_number" form:"page_number" query:"page_number"`
}

type PaginationResponse struct {
	PageSize   int `json:"page_size"`
	PageNumber int `json:"page_number"`
	Total      int `json:"total"`
}

func (p *PaginationRequest) GetPageNumber() int {
	if p.PageNumber <= 0 {
		return DefaultPageNumber
	}
	return p.PageNumber
}

func (p *PaginationRequest) GetOffset() int {
	return (p.GetPageNumber() - 1) * p.GetPageSize()
}

func (p *PaginationRequest) GetPageSize() int {
	validPageSizes := []int{1, 5, 10, 15, 25, 50}

	for _, size := range validPageSizes {
		if p.PageSize == size {
			return size
		}
	}

	return DefaultPageSize
}
