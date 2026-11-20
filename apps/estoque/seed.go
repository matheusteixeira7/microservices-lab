package main

import "apps/estoque/internal/inventory/domain"

// seedProducts is the starting catalog.
//
// TEMPORARY: the store is in memory, so stock resets on every restart. It is
// the same catalog as the old pedidos stub. Goes away with Postgres in estoque.
func seedProducts() []*domain.Product {
	brl := func(cents int64) domain.Money { return domain.Money{Cents: cents, Currency: "BRL"} }
	catalog := []struct {
		sku, name string
		price     domain.Money
		onHand    int
	}{
		{"KB-1", "Mechanical keyboard", brl(45000), 10},
		{"MS-1", "Wireless mouse", brl(15000), 25},
		{"MN-1", `27" monitor`, brl(189900), 3},
		{"OUT-1", "Always out of stock", brl(1000), 0},
	}

	products := make([]*domain.Product, 0, len(catalog))
	for _, c := range catalog {
		p, err := domain.NewProduct(c.sku, c.name, c.price, c.onHand)
		if err != nil {
			panic(err) // a broken seed is a programming error
		}
		products = append(products, p)
	}
	return products
}
