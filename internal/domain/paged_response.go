package domain

// PagedResponse представляет ответ с пагинацией для API
type PagedResponse struct {
	Items      interface{} `json:"items"`           // Элементы на текущей странице
	TotalItems int         `json:"total_items"`     // Общее количество элементов
	Page       int         `json:"page"`            // Текущая страница
	PageSize   int         `json:"page_size"`       // Размер страницы
	TotalPages int         `json:"total_pages"`     // Общее количество страниц
}