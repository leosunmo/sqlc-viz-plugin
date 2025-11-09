package main

import (
	"fmt"
	"sync"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2oracle"
)

func classesSection() string {
	return `
classes: {
  table: {
    shape: sql_table
  }
  enums: {
    grid-rows: 2
    grid-columns: 2
  }
  enum: {
    shape: sql_table
    style: {
      stroke-dash: 5
    }
  }
  views: {
    grid-rows: 2
    grid-columns: 2
  }
  view: {
    shape: sql_table
    style: {
      stroke-dash: 5
    }
  }
  domains: {
    grid-rows: 2
    grid-columns: 2
  }
  domain: {
    shape: sql_table
    style: {
      stroke-dash: 5
    }
  }
  composites: {
    grid-rows: 2
    grid-columns: 2
  }
  composite: {
    shape: sql_table
    style: {
      stroke-dash: 5
    }
  }
}`
}

var viewsKey string
var enumsKey string
var domainsKey string
var compositesKey string

func setTableClass(g *d2graph.Graph, key string) error {
	_, err := d2oracle.Set(g, nil, key+".class", nil, strPtr("table"))
	if err != nil {
		return fmt.Errorf("failed to set table class on %s: %w", key, err)
	}
	return nil
}

var enumsOnce = sync.Once{}

func createEnumCollection(g *d2graph.Graph) error {
	var err error
	enumsOnce.Do(func() {
		_, enumsKey, err = d2oracle.Create(g, nil, "enums")
		if err != nil {
			err = fmt.Errorf("failed to create enums node: %w", err)
		}

		err = setEnumCollectionClass(g, enumsKey)
	})
	return err
}
func setEnumCollectionClass(g *d2graph.Graph, key string) error {
	_, err := d2oracle.Set(g, nil, key+".class", nil, strPtr("enums"))
	if err != nil {
		return fmt.Errorf("failed to set enum collection class on %s: %w", key, err)
	}

	return nil
}

func setEnumClass(g *d2graph.Graph, key string) error {
	_, err := d2oracle.Set(g, nil, key+".class", nil, strPtr("enum"))
	if err != nil {
		return fmt.Errorf("failed to set enum class on %s: %w", key, err)
	}
	return nil
}

var viewsOnce = sync.Once{}

func createViewCollection(g *d2graph.Graph) error {
	var err error
	viewsOnce.Do(func() {
		g, viewsKey, err = d2oracle.Create(g, nil, "views")
		if err != nil {
			err = fmt.Errorf("failed to create views node: %w", err)
		}

		err = setViewCollectionClass(g, viewsKey)
	})

	return err
}

func setViewCollectionClass(g *d2graph.Graph, key string) error {
	_, err := d2oracle.Set(g, nil, key+".class", nil, strPtr("views"))
	if err != nil {
		return fmt.Errorf("failed to set view collection class on %s: %w", key, err)
	}
	return nil
}

func setViewClass(g *d2graph.Graph, key string) error {
	_, err := d2oracle.Set(g, nil, key+".class", nil, strPtr("view"))
	if err != nil {
		return fmt.Errorf("failed to set view class on %s: %w", key, err)
	}
	return nil
}

var domainsOnce = sync.Once{}

func createDomainCollection(g *d2graph.Graph) error {
	var err error
	domainsOnce.Do(func() {
		_, domainsKey, err = d2oracle.Create(g, nil, "domains")
		if err != nil {
			err = fmt.Errorf("failed to create domains node: %w", err)
		}

		err = setDomainCollectionClass(g, domainsKey)
	})
	return err
}

func setDomainCollectionClass(g *d2graph.Graph, key string) error {
	_, err := d2oracle.Set(g, nil, key+".class", nil, strPtr("domains"))
	if err != nil {
		return fmt.Errorf("failed to set domain collection class on %s: %w", key, err)
	}
	return nil
}

func setDomainClass(g *d2graph.Graph, key string) error {
	_, err := d2oracle.Set(g, nil, key+".class", nil, strPtr("domain"))
	if err != nil {
		return fmt.Errorf("failed to set domain class on %s: %w", key, err)
	}
	return nil
}

var compositesOnce = sync.Once{}

func createCompositeCollection(g *d2graph.Graph) error {
	var err error
	compositesOnce.Do(func() {
		g, compositesKey, err = d2oracle.Create(g, nil, "composites")
		if err != nil {
			err = fmt.Errorf("failed to create composites node: %w", err)
		}

		err = setCompositeCollectionClass(g, compositesKey)
	})
	return err
}

func setCompositeCollectionClass(g *d2graph.Graph, key string) error {
	_, err := d2oracle.Set(g, nil, key+".class", nil, strPtr("composite_collection"))
	if err != nil {
		return fmt.Errorf("failed to set composite collection class on %s: %w", key, err)
	}
	return nil
}

func setCompositeClass(g *d2graph.Graph, key string) error {
	_, err := d2oracle.Set(g, nil, key+".class", nil, strPtr("composite"))
	if err != nil {
		return fmt.Errorf("failed to set composite class on %s: %w", key, err)
	}
	return nil
}
