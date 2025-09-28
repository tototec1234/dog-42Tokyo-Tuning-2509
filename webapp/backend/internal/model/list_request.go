package model

import "strings"

type ListRequestConfig struct {
    DefaultPage       int
    DefaultPageSize   int
    MaxPageSize       int
    DefaultSortField  string
    DefaultSortOrder  string
    AllowedSortFields []string
    AllowedSortOrders []string
    DefaultType       string
    AllowedTypes      []string
}

func (r *ListRequest) Normalize(cfg ListRequestConfig) {
    if cfg.DefaultPage <= 0 {
        cfg.DefaultPage = 1
    }
    if cfg.DefaultPageSize <= 0 {
        cfg.DefaultPageSize = 20
    }
    if cfg.MaxPageSize <= 0 {
        cfg.MaxPageSize = cfg.DefaultPageSize
    }

    if r.Page <= 0 {
        r.Page = cfg.DefaultPage
    }

    if r.PageSize <= 0 {
        r.PageSize = cfg.DefaultPageSize
    } else if r.PageSize > cfg.MaxPageSize {
        r.PageSize = cfg.MaxPageSize
    }

    if len(cfg.AllowedSortFields) > 0 {
        allowed := make(map[string]struct{}, len(cfg.AllowedSortFields))
        for _, field := range cfg.AllowedSortFields {
            allowed[strings.ToLower(field)] = struct{}{}
        }
        field := strings.ToLower(r.SortField)
        if _, ok := allowed[field]; !ok {
            fallback := strings.ToLower(cfg.DefaultSortField)
            if _, ok := allowed[fallback]; ok {
                field = fallback
            } else if len(cfg.AllowedSortFields) > 0 {
                field = strings.ToLower(cfg.AllowedSortFields[0])
            }
        }
        r.SortField = field
    } else if cfg.DefaultSortField != "" && r.SortField == "" {
        r.SortField = strings.ToLower(cfg.DefaultSortField)
    }

    allowedOrders := cfg.AllowedSortOrders
    if len(allowedOrders) == 0 {
        allowedOrders = []string{"asc", "desc"}
    }
    allowedOrderSet := make(map[string]struct{}, len(allowedOrders))
    for _, order := range allowedOrders {
        allowedOrderSet[strings.ToUpper(order)] = struct{}{}
    }

    order := strings.ToUpper(r.SortOrder)
    if _, ok := allowedOrderSet[order]; !ok {
        fallback := strings.ToUpper(cfg.DefaultSortOrder)
        if _, ok := allowedOrderSet[fallback]; ok {
            order = fallback
        } else {
            for o := range allowedOrderSet {
                order = o
                break
            }
        }
    }
    r.SortOrder = order

    if len(cfg.AllowedTypes) > 0 {
        allowedTypeSet := make(map[string]struct{}, len(cfg.AllowedTypes))
        for _, t := range cfg.AllowedTypes {
            allowedTypeSet[strings.ToLower(t)] = struct{}{}
        }
        reqType := strings.ToLower(r.Type)
        if _, ok := allowedTypeSet[reqType]; !ok {
            defaultType := strings.ToLower(cfg.DefaultType)
            if _, ok := allowedTypeSet[defaultType]; ok {
                reqType = defaultType
            } else if len(cfg.AllowedTypes) > 0 {
                reqType = strings.ToLower(cfg.AllowedTypes[0])
            }
        }
        r.Type = reqType
    }

    if r.Page < 1 {
        r.Page = 1
    }
    if r.PageSize < 1 {
        r.PageSize = cfg.DefaultPageSize
    }

    r.Offset = (r.Page - 1) * r.PageSize
}


