package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"
	"github.com/spf13/pflag"
	"github.com/sqlc-dev/plugin-sdk-go/codegen"
	pb "github.com/sqlc-dev/plugin-sdk-go/plugin"

	"oss.terrastruct.com/d2/d2format"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2layouts/d2elklayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2oracle"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	d2log "oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

type Column struct {
	Name       string
	Type       string
	PrimaryKey bool
	Unique     bool
	ForeignKey *FK // optional
}
type Table struct {
	Schema string
	Name   string
	Cols   []Column
}
type FK struct {
	SrcCols             []string
	DstSchema, DstTable string
	DstCols             []string
}

var migrationDir = pflag.StringP("migrations", "m", "", "path to migration files or directory")

func main() {
	pflag.Parse()
	if *migrationDir != "" {
		err := runLocal()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed: %s", err.Error())
			os.Exit(1)
		}
		return
	}
	// If no migration path was set, it's running as a plugin.
	runPlugin()
}

func runLocal() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	files := walkMigrations([]string{"migrations"})
	if len(files) == 0 {
		return fmt.Errorf("unable to find any schemas")
	}

	ctx = d2log.With(ctx, slog.New(slog.NewTextHandler(os.Stdout, nil)))

	f, err := run(ctx, files)
	if err != nil {
		return err
	}

	return writeFiles(f)
}

func writeFiles(files []file) error {
	for _, f := range files {
		err := os.WriteFile(f.path, []byte(f.content), 0644)
		if err != nil {
			return fmt.Errorf("failed to write file %s: %w", f.path, err)
		}
	}
	return nil
}

type file struct {
	path    string
	content string
}

func run(ctx context.Context, files []string) ([]file, error) {
	// Keep sqlc’s lexicographic ordering behavior
	sort.Strings(files)

	tables, tableLevelFKs, err := parseFiles(files)
	if err != nil {
		return nil, fmt.Errorf("failed to parse files: %s", err)
	}

	g, err := renderD2(tables, *tableLevelFKs)
	if err != nil {
		return nil, fmt.Errorf("failed to render d2: %s", err)
	}

	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("failed to create text ruler: %w", err)
	}
	if ruler == nil {
		return nil, fmt.Errorf("text ruler was nil")
	}

	gf := d2format.Format(g.AST)

	lr := func(engine string) (d2graph.LayoutGraph, error) {
		switch engine {
		case "elk":
			return d2elklayout.DefaultLayout, nil
		case "dagre":
			return d2dagrelayout.DefaultLayout, nil
		default:
			return nil, fmt.Errorf("unknown layout engine: %s", engine)
		}
	}

	// themeID := int64(d2themescatalog.DarkFlagshipTerrastruct.ID)
	themeID := int64(d2themescatalog.NeutralDefault.ID)
	// Compile D2 -> diagram

	diagram, _, err := d2lib.Compile(ctx, gf,
		&d2lib.CompileOptions{
			LayoutResolver: lr,
			Layout:         strPtr("elk"),
			Ruler:          ruler,
		},
		&d2svg.RenderOpts{ThemeID: &themeID},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to compile d2: %w", err)
	}

	// Render diagram -> SVG bytes
	svg, err := d2svg.Render(diagram, &d2svg.RenderOpts{
		ThemeID: &themeID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render svg: %w", err)
	}

	fs := []file{
		{
			path:    "schema.svg",
			content: string(svg),
		},
		{
			path:    "schema.d2",
			content: gf,
		},
	}

	return fs, nil
}

func runPlugin() {
	codegen.Run(func(ctx context.Context, gr *pb.GenerateRequest) (*pb.GenerateResponse, error) {
		// We need to add discard logger to suppress d2 logs to stdout as it
		// interferes with the plugin protocol.
		ctx = d2log.With(ctx, slog.New(slog.DiscardHandler))

		files := walkMigrations(gr.Settings.Schema)
		if len(files) == 0 {
			return &pb.GenerateResponse{}, fmt.Errorf("unable to find any schemas")
		}

		f, err := run(ctx, files)
		if err != nil {
			return &pb.GenerateResponse{}, err
		}

		var respFiles []*pb.File
		for _, fi := range f {
			respFiles = append(respFiles, &pb.File{
				Name:     fi.path,
				Contents: []byte(fi.content),
			})
		}

		return &pb.GenerateResponse{
			Files: respFiles,
		}, nil
	})
}

func parseFiles(paths []string) (tables map[string]*Table, tableLevelFKs *[]FK, err error) {
	tables = map[string]*Table{}
	tableLevelFKs = &[]FK{}
	for _, path := range paths {
		err := parseFile(path, tables, tableLevelFKs)
		if err != nil {
			return nil, nil, err
		}
	}
	return tables, tableLevelFKs, nil
}

func parseFile(path string, tables map[string]*Table, tableLevelFKs *[]FK) error {
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
			_ = ensureTable(tables, sch, tn)
			for _, n := range at.GetCmds() {
				cmd := n.GetAlterTableCmd()
				if cmd == nil {
					continue
				}
				if cmd.GetSubtype() != pgquery.AlterTableType_AT_AddConstraint {
					continue
				}
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
				}
			}
		}
	}
	return nil
}

func renderD2(tables map[string]*Table, tlfks []FK) (*d2graph.Graph, error) {
	// var b bytes.Buffer
	// blocks
	ks := make([]string, 0, len(tables))
	for k := range tables {
		ks = append(ks, k)
	}
	sort.Strings(ks)

	_, g, _ := d2lib.Compile(context.Background(), "", nil, nil)

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
		var err error
		shape := "sql_table"
		g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.shape", title), nil, &shape)
		if err != nil {
			return nil, fmt.Errorf("failed to set sql_table shape on %s: %w", title, err)
		}
		// fmt.Fprintf(&b, "%s: {\n  shape: sql_table\n", title)
		for _, c := range t.Cols {
			typ := c.Type
			typ, _ = strings.CutPrefix(typ, "pg_catalog.")

			// line := fmt.Sprintf("  %s: %q", c.Name, c.Type)
			g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.%s", title, c.Name), nil, &typ)
			if err != nil {
				return nil, fmt.Errorf("failed to set column type on %s.%s: %w", title, c.Name, err)
			}
			var cons []string
			if c.PrimaryKey {
				cons = append(cons, "primary_key")
			}
			if c.Unique {
				cons = append(cons, "unique")
			}
			if c.ForeignKey != nil {
				cons = append(cons, "foreign_key")
			}
			if len(cons) > 0 {
				// line += fmt.Sprintf(" {constraint: %s}", strings.Join(cons, " "))
				g, err = d2oracle.Set(g, nil, fmt.Sprintf("%s.%s.constraint", title, c.Name), nil, strPtr(strings.Join(cons, " ")))
				if err != nil {
					return nil, fmt.Errorf("failed to set column constraints on %s.%s: %w", title, c.Name, err)
				}
			}
			// b.WriteString(line + "\n")
		}
		// b.WriteString("}\n\n")
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
				// fmt.Fprintf(&b, "%s.%s -> %s.%s\n", left, c.Name, right, dstCol)
			} else {
				// fmt.Fprintf(&b, "%s.%s -> %s\n", left, c.Name, right)
				g, _, _ = d2oracle.Create(g, nil, ""+left+"."+c.Name+" -> "+right)
			}
		}
	}

	// table-level FK edges (pair cols if possible; else table edge)
	for _, fk := range tlfks {
		right := tableLabel(fk.DstSchema, fk.DstTable)
		for _, t := range tables {
			if hasAllCols(t, fk.SrcCols) {
				left := tableLabel(t.Schema, t.Name)
				if len(fk.SrcCols) == len(fk.DstCols) && len(fk.SrcCols) > 0 {
					for i := range fk.SrcCols {
						g, _, _ = d2oracle.Create(g, nil, ""+left+"."+fk.SrcCols[i]+" -> "+right+"."+fk.DstCols[i])
						// fmt.Fprintf(&b, "%s.%s -> %s.%s\n", left, fk.SrcCols[i], right, fk.DstCols[i])
					}
				} else {
					// fmt.Fprintf(&b, "%s -> %s\n", left, right)
					g, _, _ = d2oracle.Create(g, nil, ""+left+" -> "+right)
				}
				break
			}
		}
	}
	// return b.String(), nil
	return g, nil
}

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

func strPtr(s string) *string {
	return &s
}

func walkMigrations(migPaths []string) []string {
	var files []string
	for _, p := range migPaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				// Keep .sql files; prefer “*.up.sql” if you use goose/migrate/tern.
				name := strings.ToLower(d.Name())
				if strings.HasSuffix(name, ".sql") {
					// If you only want ups, uncomment the next line:
					// if !strings.Contains(name, ".up.") { return nil }
					files = append(files, path)
				}
				return nil
			})
		} else {
			if strings.HasSuffix(strings.ToLower(p), ".sql") {
				files = append(files, p)
			}
		}
	}
	return files
}
