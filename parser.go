package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"
)

func parseFiles(paths []string) (tables map[string]*Table, tableLevelFKs *[]FK, views map[string]*View, customTypes map[string]*CustomType, err error) {
	tables = map[string]*Table{}
	tableLevelFKs = &[]FK{}
	views = map[string]*View{}
	customTypes = map[string]*CustomType{}
	for _, path := range paths {
		err := parseSQL(path, tables, tableLevelFKs, views, customTypes)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}
	return tables, tableLevelFKs, views, customTypes, nil
}

func parseSQL(path string, tables map[string]*Table, tableLevelFKs *[]FK, views map[string]*View, customTypes map[string]*CustomType) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	pieces := bytes.SplitN(b, []byte("---- create above / drop below ----"), 2)
	up := pieces[0]

	res, err := pgquery.Parse(string(up))
	if err != nil {
		return fmt.Errorf("failed to parse SQL in %s: %w", path, err)
	}

	for _, raw := range res.GetStmts() {
		s := raw.GetStmt()

		if cs := s.GetCreateStmt(); cs != nil {
			sch := getSchema(cs.GetRelation())
			tn := cs.GetRelation().GetRelname()
			if tn == "" {
				continue
			}
			t := ensureTable(tables, sch, tn)

			for _, elt := range cs.GetTableElts() {
				if cd := elt.GetColumnDef(); cd != nil {
					col := Column{Name: cd.GetColname(), Type: typeName(cd.GetTypeName())}
					for _, rc := range cd.GetConstraints() {
						c := rc.GetConstraint()
						switch c.Contype {
						case pgquery.ConstrType_CONSTR_PRIMARY:
							col.PrimaryKey = true
						case pgquery.ConstrType_CONSTR_UNIQUE:
							col.Unique = true
						case pgquery.ConstrType_CONSTR_FOREIGN:
							dstS, dstT := pktable(c)
							col.ForeignKey = &FK{
								SrcCols:   []string{col.Name},
								DstSchema: dstS, DstTable: dstT,
								DstCols: nodeIdents(c.GetPkAttrs()),
							}
						}
					}
					t.Cols = upsertCol(t.Cols, col)
				}
				if c := elt.GetConstraint(); c != nil {
					switch c.GetContype() {
					case pgquery.ConstrType_CONSTR_PRIMARY:
						for _, n := range nodeIdents(c.GetKeys()) {
							markPK(t, n)
						}
					case pgquery.ConstrType_CONSTR_UNIQUE:
						for _, n := range nodeIdents(c.GetKeys()) {
							markUQ(t, n)
						}
					case pgquery.ConstrType_CONSTR_FOREIGN:
						dstS, dstT := pktable(c)
						*tableLevelFKs = append(*tableLevelFKs, FK{
							SrcCols:   nodeIdents(c.GetFkAttrs()),
							DstSchema: dstS, DstTable: dstT,
							DstCols: nodeIdents(c.GetPkAttrs()),
						})
					case pgquery.ConstrType_CONSTR_CHECK:
						if re := c.GetRawExpr(); re != nil {
							constraint := TableConstraint{
								Name:        c.GetConname(),
								Type:        "CHECK",
								Description: extractNodeConstraint(re),
							}
							t.Constraints = append(t.Constraints, constraint)
						}
					}
				}
			}
			continue
		}

		if at := s.GetAlterTableStmt(); at != nil {
			sch := getSchema(at.GetRelation())
			tn := at.GetRelation().GetRelname()
			if tn == "" {
				continue
			}
			t := ensureTable(tables, sch, tn)
			for _, n := range at.GetCmds() {
				cmd := n.GetAlterTableCmd()
				if cmd == nil {
					continue
				}
				switch cmd.GetSubtype() {
				case pgquery.AlterTableType_AT_AddColumn:
					// Handle ADD COLUMN
					if cd := cmd.GetDef().GetColumnDef(); cd != nil {
						col := Column{Name: cd.GetColname(), Type: typeName(cd.GetTypeName())}
						for _, rc := range cd.GetConstraints() {
							c := rc.GetConstraint()
							switch c.Contype {
							case pgquery.ConstrType_CONSTR_PRIMARY:
								col.PrimaryKey = true
							case pgquery.ConstrType_CONSTR_UNIQUE:
								col.Unique = true
							case pgquery.ConstrType_CONSTR_FOREIGN:
								dstS, dstT := pktable(c)
								col.ForeignKey = &FK{
									SrcCols:   []string{col.Name},
									DstSchema: dstS, DstTable: dstT,
									DstCols: nodeIdents(c.GetPkAttrs()),
								}
							}
						}
						t.Cols = upsertCol(t.Cols, col)
					}
				case pgquery.AlterTableType_AT_DropColumn:
					// Handle DROP COLUMN
					if cmd.GetName() != "" {
						t.Cols = removeCol(t.Cols, cmd.GetName())
					}
				case pgquery.AlterTableType_AT_AddConstraint:
					con := cmd.GetDef().GetConstraint()
					if con == nil {
						continue
					}
					switch con.GetContype() {
					case pgquery.ConstrType_CONSTR_PRIMARY:
						for _, k := range nodeIdents(con.GetKeys()) {
							markPK(tables[key(sch, tn)], k)
						}
					case pgquery.ConstrType_CONSTR_UNIQUE:
						for _, k := range nodeIdents(con.GetKeys()) {
							markUQ(tables[key(sch, tn)], k)
						}
					case pgquery.ConstrType_CONSTR_FOREIGN:
						dstS, dstT := pktable(con)
						*tableLevelFKs = append(*tableLevelFKs, FK{
							SrcCols:   nodeIdents(con.GetFkAttrs()),
							DstSchema: dstS, DstTable: dstT,
							DstCols: nodeIdents(con.GetPkAttrs()),
						})
					case pgquery.ConstrType_CONSTR_CHECK:
						if re := con.GetRawExpr(); re != nil {
							constraint := TableConstraint{
								Name:        con.GetConname(),
								Type:        "CHECK",
								Description: extractNodeConstraint(re),
							}
							t := tables[key(sch, tn)]
							if t != nil {
								t.Constraints = append(t.Constraints, constraint)
							}
						}
					}
				}
			}
		}

		if vs := s.GetViewStmt(); vs != nil {
			// Handle CREATE VIEW
			sch := getSchema(vs.GetView())
			vn := vs.GetView().GetRelname()
			if vn != "" {
				view := &View{
					Schema: sch,
					Name:   vn,
				}

				// Try to extract columns from the SELECT statement
				if query := vs.GetQuery(); query != nil {
					cols := extractViewColumns(query)
					view.Cols = cols
				}

				views[key(sch, vn)] = view
			}
		}

		if cts := s.GetCompositeTypeStmt(); cts != nil {
			// Handle CREATE TYPE (composite types)
			sch := getSchema(cts.GetTypevar())
			tn := cts.GetTypevar().GetRelname()
			if tn != "" {
				ct := &CustomType{
					Schema:   sch,
					Name:     tn,
					TypeKind: "composite",
				}

				// Extract columns from composite type
				for _, col := range cts.GetColdeflist() {
					if cd := col.GetColumnDef(); cd != nil {
						column := Column{
							Name: cd.GetColname(),
							Type: typeName(cd.GetTypeName()),
						}
						ct.Cols = append(ct.Cols, column)
					}
				}

				customTypes[key(sch, tn)] = ct
			}
		}

		if ets := s.GetCreateEnumStmt(); ets != nil {
			// Handle CREATE TYPE ... AS ENUM
			if len(ets.GetTypeName()) > 0 {
				names := nodeIdents(ets.GetTypeName())
				sch := ""
				tn := names[0]
				if len(names) > 1 {
					sch = names[0]
					tn = names[1]
				}
				values := make([]string, 0, len(ets.GetVals()))
				for _, val := range ets.GetVals() {
					if s := val.GetString_(); s != nil {
						values = append(values, s.GetSval())
					}
				}
				customTypes[key(sch, tn)] = &CustomType{
					Schema:   sch,
					Name:     tn,
					TypeKind: "enum",
					Values:   values,
				}
			}
		}

		if dds := s.GetCreateDomainStmt(); dds != nil {
			// Handle CREATE DOMAIN
			if len(dds.GetDomainname()) > 0 {
				names := nodeIdents(dds.GetDomainname())
				sch := ""
				dn := names[0]
				if len(names) > 1 {
					sch = names[0]
					dn = names[1]
				}

				ct := &CustomType{
					Schema:   sch,
					Name:     dn,
					TypeKind: "domain",
					BaseType: typeName(dds.GetTypeName()),
				}

				// Extract domain constraints
				for _, constraint := range dds.GetConstraints() {
					if c := constraint.GetConstraint(); c != nil {
						switch c.GetContype() {
						case pgquery.ConstrType_CONSTR_CHECK:
							// Extract a human-readable constraint description
							if re := c.GetRawExpr(); re != nil {
								ct.Check = extractNodeConstraint(re)
							}
						case pgquery.ConstrType_CONSTR_NOTNULL:
							ct.NotNull = true
						}
					}
				}

				// Extract collation if present
				if collation := dds.GetCollClause(); collation != nil {
					if len(collation.GetCollname()) > 0 {
						collationNames := nodeIdents(collation.GetCollname())
						ct.Collation = strings.Join(collationNames, ".")
					}
				}

				customTypes[key(sch, dn)] = ct
			}
		}

		if ds := s.GetDropStmt(); ds != nil {
			// Handle DROP TABLE, DROP VIEW, etc.
			switch ds.GetRemoveType() {
			case pgquery.ObjectType_OBJECT_TABLE:
				for _, obj := range ds.GetObjects() {
					if list := obj.GetList(); list != nil {
						if names := nodeIdents(list.GetItems()); len(names) > 0 {
							sch := ""
							tn := names[0]
							if len(names) > 1 {
								sch = names[0]
								tn = names[1]
							}
							delete(tables, key(sch, tn))
						}
					}
				}
			case pgquery.ObjectType_OBJECT_VIEW:
				for _, obj := range ds.GetObjects() {
					if list := obj.GetList(); list != nil {
						if names := nodeIdents(list.GetItems()); len(names) > 0 {
							sch := ""
							vn := names[0]
							if len(names) > 1 {
								sch = names[0]
								vn = names[1]
							}
							delete(views, key(sch, vn))
						}
					}
				}
			case pgquery.ObjectType_OBJECT_TYPE:
				for _, obj := range ds.GetObjects() {
					if list := obj.GetList(); list != nil {
						if names := nodeIdents(list.GetItems()); len(names) > 0 {
							sch := ""
							tn := names[0]
							if len(names) > 1 {
								sch = names[0]
								tn = names[1]
							}
							delete(customTypes, key(sch, tn))
						}
					}
				}
			case pgquery.ObjectType_OBJECT_DOMAIN:
				for _, obj := range ds.GetObjects() {
					if list := obj.GetList(); list != nil {
						if names := nodeIdents(list.GetItems()); len(names) > 0 {
							sch := ""
							dn := names[0]
							if len(names) > 1 {
								sch = names[0]
								dn = names[1]
							}
							delete(customTypes, key(sch, dn))
						}
					}
				}
			}
		}
	}
	return nil
}

func extractViewColumns(query *pgquery.Node) []Column {
	var cols []Column

	// Handle SELECT statement
	if sel := query.GetSelectStmt(); sel != nil {
		for _, target := range sel.GetTargetList() {
			if rt := target.GetResTarget(); rt != nil {
				colName := rt.GetName()
				if colName == "" {
					// If no alias, try to infer from the expression
					if colRef := rt.GetVal().GetColumnRef(); colRef != nil {
						if fields := colRef.GetFields(); len(fields) > 0 {
							if str := fields[len(fields)-1].GetString_(); str != nil {
								colName = str.GetSval()
							}
						}
					} else if funcCall := rt.GetVal().GetFuncCall(); funcCall != nil {
						// For function calls like count(*), use the function name
						if funcName := funcCall.GetFuncname(); len(funcName) > 0 {
							if str := funcName[len(funcName)-1].GetString_(); str != nil {
								colName = str.GetSval()
							}
						}
					}
				}

				if colName != "" {
					cols = append(cols, Column{
						Name: colName,
						Type: "unknown", // We can't easily determine the type from the AST
					})
				}
			}
		}
	}

	return cols
}

// Utility functions for parsing
func getSchema(rv *pgquery.RangeVar) string {
	if rv == nil {
		return ""
	}
	return rv.GetSchemaname()
}

func pktable(c *pgquery.Constraint) (string, string) {
	if c == nil || c.GetPktable() == nil {
		return "", ""
	}
	return c.GetPktable().GetSchemaname(), c.GetPktable().GetRelname()
}

func typeName(tn *pgquery.TypeName) string {
	if tn == nil {
		return ""
	}
	parts := nodeIdents(tn.GetNames())
	typ := strings.Join(parts, ".")
	if l := tn.GetTypmods(); len(l) > 0 {
		var mods []string
		for _, m := range l {
			if ival := m.GetAConst().GetIval(); ival != nil {
				mods = append(mods, fmt.Sprintf("%d", ival.GetIval()))
			}
		}
		if len(mods) > 0 {
			typ += "(" + strings.Join(mods, ", ") + ")"
		}
	}
	if len(tn.GetArrayBounds()) > 0 {
		typ += "[]"
	}
	return typ
}

func nodeIdents(nodes []*pgquery.Node) []string {
	var out []string
	for _, n := range nodes {
		if s := n.GetString_(); s != nil {
			out = append(out, s.GetSval())
		}
	}
	return out
}

// Table manipulation utilities
func key(s, t string) string {
	if s == "" {
		return t
	}
	return s + "." + t
}

func ensureTable(m map[string]*Table, s, t string) *Table {
	k := key(s, t)
	if m[k] == nil {
		m[k] = &Table{Schema: s, Name: t}
	}
	return m[k]
}

func upsertCol(cols []Column, c Column) []Column {
	for i := range cols {
		if cols[i].Name == c.Name {
			if c.Type != "" {
				cols[i].Type = c.Type
			}
			cols[i].PrimaryKey = cols[i].PrimaryKey || c.PrimaryKey
			cols[i].Unique = cols[i].Unique || c.Unique
			if c.ForeignKey != nil {
				cols[i].ForeignKey = c.ForeignKey
			}
			return cols
		}
	}
	return append(cols, c)
}

func removeCol(cols []Column, name string) []Column {
	result := make([]Column, 0, len(cols))
	for _, col := range cols {
		if col.Name != name {
			result = append(result, col)
		}
	}
	return result
}

func markPK(t *Table, name string) {
	for i := range t.Cols {
		if t.Cols[i].Name == name {
			t.Cols[i].PrimaryKey = true
		}
	}
}

func markUQ(t *Table, name string) {
	for i := range t.Cols {
		if t.Cols[i].Name == name {
			t.Cols[i].Unique = true
		}
	}
}

func hasAllCols(t *Table, want []string) bool {
	have := map[string]bool{}
	for _, c := range t.Cols {
		have[c.Name] = true
	}
	for _, w := range want {
		if !have[w] {
			return false
		}
	}
	return true
}

func tableLabel(schema, name string) string {
	if name == "" {
		return ""
	}
	if schema == "" || schema == "public" {
		return name
	}
	return schema + "." + name
}
