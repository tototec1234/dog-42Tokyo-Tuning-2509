package repository

import "strings"

func normalizeSortOrder(order, defaultOrder string) string {
    order = strings.ToUpper(order)
    if order != "ASC" && order != "DESC" {
        return strings.ToUpper(defaultOrder)
    }
    return order
}

func sanitizeSortField(field string, defaultField string, allowed map[string]string) string {
    normalized := strings.ToLower(field)
    if mapped, ok := allowed[normalized]; ok {
        return mapped
    }

    defaultKey := strings.ToLower(defaultField)
    if mapped, ok := allowed[defaultKey]; ok {
        return mapped
    }

    for _, v := range allowed {
        return v
    }
    return ""
}


