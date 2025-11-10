package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2oracle"
)

// renderD2 creates a D2 graph representation of the database schema
func renderD2(tables map[string]*Table, tlfk []FK, views map[string]*View, customTypes map[string]*CustomType) (*d2graph.Graph, error) {
	ks := make([]string, 0, len(tables))
	for k := range tables {
		ks = append(ks, k)
	}
	sort.Strings(ks)

	// initialise with classes section
	_, g, _ := d2lib.Compile(context.Background(), classesSection(), nil, nil)

	var err error

	for _, k := range ks {
		t := tables[k]
		title := t.Name
		if t.Schema != "" && t.Schema != "public" {
			title = t.Schema + "." + t.Name
		}
		if g == nil {
			panic("g was nil")
		}
		g, _, _ = d2oracle.Create(g, nil, title)
		err = setTableClass(g, title)
		if err != nil {
			return nil, err
		}
		for _, c := range t.Cols {
			typ := c.Type
			typ, _ = strings.CutPrefix(typ, "pg_catalog.")

			g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.%s", title, c.Name), nil, &typ)
			if err != nil {
				return nil, fmt.Errorf("failed to set column type on %s.%s: %w", title, c.Name, err)
			}
			var cons []string
			if c.PrimaryKey {
				cons = append(cons, "PK")
			}
			if c.Unique {
				cons = append(cons, "UNQ")
			}
			if c.ForeignKey != nil {
				cons = append(cons, "FK")
			}
			if len(cons) > 0 {
				g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.%s.constraint", title, c.Name), nil, strPtr(strings.Join(cons, " ")))
				if err != nil {
					return nil, fmt.Errorf("failed to set column constraints on %s.%s: %w", title, c.Name, err)
				}
			}
		}

		// Add table-level constraints
		for _, constraint := range t.Constraints {
			constraintName := constraint.Name
			if constraintName == "" {
				constraintName = fmt.Sprintf("check_%d", len(t.Constraints))
			}
			g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.%s", title, constraintName), nil, strPtr(constraint.Description))
			if err != nil {
				return nil, fmt.Errorf("failed to set table constraint on %s.%s: %w", title, constraintName, err)
			}
		}
	}

	// column-level FK edges
	for _, t := range tables {
		left := tableLabel(t.Schema, t.Name)
		for _, c := range t.Cols {
			if c.ForeignKey == nil {
				continue
			}
			right := tableLabel(c.ForeignKey.DstSchema, c.ForeignKey.DstTable)
			dstCol := ""
			if len(c.ForeignKey.DstCols) > 0 {
				dstCol = c.ForeignKey.DstCols[0]
			}
			if dstCol != "" {
				g, _, _ = d2oracle.Create(g, nil, ""+left+"."+c.Name+" -> "+right+"."+dstCol)
			} else {
				g, _, _ = d2oracle.Create(g, nil, ""+left+"."+c.Name+" -> "+right)
			}
		}
	}

	// table-level FK edges (pair cols if possible; else table edge)
	for _, fk := range tlfk {
		right := tableLabel(fk.DstSchema, fk.DstTable)
		for _, t := range tables {
			if hasAllCols(t, fk.SrcCols) {
				left := tableLabel(t.Schema, t.Name)
				if len(fk.SrcCols) == len(fk.DstCols) && len(fk.SrcCols) > 0 {
					for i := range fk.SrcCols {
						g, _, _ = d2oracle.Create(g, nil, ""+left+"."+fk.SrcCols[i]+" -> "+right+"."+fk.DstCols[i])
					}
				} else {
					g, _, _ = d2oracle.Create(g, nil, ""+left+" -> "+right)
				}
				break
			}
		}
	}

	// views
	vks := make([]string, 0, len(views))
	for k := range views {
		vks = append(vks, k)
	}
	sort.Strings(vks)

	err = createViewCollection(g)
	if err != nil {
		return nil, err
	}

	for _, k := range vks {
		v := views[k]
		schema := v.Schema
		if schema != "" && schema != "public" {
			schema = v.Schema + "." + v.Name
		}
		var title string
		if schema != "" {
			title = fmt.Sprintf("%s.%s.%s", viewsKey, schema, v.Name)
		} else {
			title = fmt.Sprintf("%s.%s", viewsKey, v.Name)
		}
		g, _, _ = d2oracle.Create(g, nil, title)
		var err error
		err = setViewClass(g, title)
		if err != nil {
			return nil, err
		}

		// Add view columns
		for _, c := range v.Cols {
			typ := c.Type
			if typ == "unknown" {
				typ = "" // Don't show unknown types
			}
			g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.%s", title, c.Name), nil, &typ)
			if err != nil {
				return nil, fmt.Errorf("failed to set view column on %s.%s: %w", title, c.Name, err)
			}
		}
	}

	// custom types
	ctks := make([]string, 0, len(customTypes))
	for k := range customTypes {
		ctks = append(ctks, k)
	}
	sort.Strings(ctks)

	for _, k := range ctks {
		ct := customTypes[k]
		title := ct.Name
		if ct.Schema != "" && ct.Schema != "public" {
			title = ct.Schema + "." + ct.Name
		}
		var err error

		// Set different styles for different type kinds
		switch ct.TypeKind {
		case "enum":
			err = createEnumCollection(g)
			if err != nil {
				return nil, err
			}
			g, title, err = d2oracle.Create(g, nil, fmt.Sprintf("%s.%s", enumsKey, title))
			if err != nil {
				return nil, fmt.Errorf("failed to create enum %s: %w", title, err)
			}
			// Add enum values as "columns"
			for _, value := range ct.Values {
				g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.%s", title, value), nil, strPtr(""))
				if err != nil {
					return nil, fmt.Errorf("failed to set enum value on %s.%s: %w", title, value, err)
				}
			}
			err = setEnumClass(g, title)
			if err != nil {
				return nil, err
			}

		case "domain":
			err = createDomainCollection(g)
			if err != nil {
				return nil, err
			}
			title = fmt.Sprintf("%s.%s", domainsKey, title)

			g, title, err = d2oracle.Create(g, nil, title)
			if err != nil {
				return nil, fmt.Errorf("failed to create domain %s: %w", title, err)
			}

			// Show base type and constraints
			baseTypeLabel := "base_type"
			g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.%s", title, baseTypeLabel), nil, &ct.BaseType)
			if err != nil {
				return nil, fmt.Errorf("failed to set base type on %s: %w", title, err)
			}
			if ct.Check != "" {
				g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.constraints", title), nil, strPtr(ct.Check))
				if err != nil {
					return nil, fmt.Errorf("failed to set constraints on %s: %w", title, err)
				}
			}
			err = setDomainClass(g, title)
			if err != nil {
				return nil, err
			}

		case "composite":
			err = createCompositeCollection(g)
			if err != nil {
				return nil, err
			}
			g, title, err = d2oracle.Create(g, nil, fmt.Sprintf("%s.%s", compositesKey, title))
			if err != nil {
				return nil, fmt.Errorf("failed to create composite type %s: %w", title, err)
			}
			// Add composite type columns
			for _, col := range ct.Cols {
				g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.%s", title, col.Name), nil, &col.Type)
				if err != nil {
					return nil, fmt.Errorf("failed to set composite column on %s.%s: %w", title, col.Name, err)
				}
			}
			err = setCompositeClass(g, title)
			if err != nil {
				return nil, err
			}
		}
	}

	// // Add relationships from tables/views to custom types they use
	// g, err = addCustomTypeRelationships(g, tables, views, customTypes)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to add custom type relationships: %w", err)
	// }

	return g, nil
}

func addCustomTypeRelationships(g *d2graph.Graph, tables map[string]*Table, views map[string]*View, customTypes map[string]*CustomType) (*d2graph.Graph, error) {
	// Track which custom types are used
	usedTypes := make(map[string]bool)

	// Check table columns for custom type usage
	for _, table := range tables {
		tableTitle := table.Name
		if table.Schema != "" && table.Schema != "public" {
			tableTitle = table.Schema + "." + table.Name
		}

		for _, col := range table.Cols {
			// Check if the column type matches any custom type
			for ctKey, ct := range customTypes {
				if col.Type == ct.Name || col.Type == ctKey {
					usedTypes[ctKey] = true
					// Create relationship from table column to custom type
					ctTitle := ct.Name
					if ct.Schema != "" && ct.Schema != "public" {
						ctTitle = ct.Schema + "." + ct.Name
					}
					t := fmt.Sprintf("%ss.%s", ct.TypeKind, ctTitle)
					g, _, _ = d2oracle.Create(g, nil, fmt.Sprintf("%s.%s -> %s", tableTitle, col.Name, t))
				}
			}
		}
	}

	// Check view columns for custom type usage
	for _, view := range views {
		viewTitle := view.Name
		if view.Schema != "" && view.Schema != "public" {
			viewTitle = view.Schema + "." + view.Name
		}

		for _, col := range view.Cols {
			// Check if the column type matches any custom type
			for ctKey, ct := range customTypes {
				if col.Type == ct.Name || col.Type == ctKey {
					usedTypes[ctKey] = true
					// Create relationship from view column to custom type
					ctTitle := ct.Name
					if ct.Schema != "" && ct.Schema != "public" {
						ctTitle = ct.Schema + "." + ct.Name
					}
					g, _, _ = d2oracle.Create(g, nil, fmt.Sprintf("%s.%s -> %s", viewTitle, col.Name, ctTitle))
				}
			}
		}
	}

	// Also check for composite types used within other composite types
	for _, ct := range customTypes {
		if ct.TypeKind == "composite" {
			ctTitle := ct.Name
			if ct.Schema != "" && ct.Schema != "public" {
				ctTitle = ct.Schema + "." + ct.Name
			}

			for _, col := range ct.Cols {
				for otherCtKey, otherCt := range customTypes {
					if col.Type == otherCt.Name || col.Type == otherCtKey {
						usedTypes[otherCtKey] = true
						// Create relationship from composite type field to other custom type
						otherCtTitle := otherCt.Name
						if otherCt.Schema != "" && otherCt.Schema != "public" {
							otherCtTitle = otherCt.Schema + "." + otherCt.Name
						}
						g, _, _ = d2oracle.Create(g, nil, fmt.Sprintf("%s.%s -> %s", ctTitle, col.Name, otherCtTitle))
					}
				}
			}
		}
	}

	return g, nil
}

func strPtr(s string) *string {
	return &s
}
