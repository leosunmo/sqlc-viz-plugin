package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/pflag"
	"github.com/sqlc-dev/plugin-sdk-go/codegen"
	pb "github.com/sqlc-dev/plugin-sdk-go/plugin"

	"oss.terrastruct.com/d2/d2format"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2layouts/d2elklayout"
	"oss.terrastruct.com/d2/d2lib"
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
	Schema      string
	Name        string
	Cols        []Column
	Constraints []TableConstraint
}

type TableConstraint struct {
	Name        string
	Type        string // "CHECK", "UNIQUE", "PRIMARY", etc.
	Description string
}
type FK struct {
	SrcCols             []string
	DstSchema, DstTable string
	DstCols             []string
}
type View struct {
	Schema string
	Name   string
	Query  string
	Cols   []Column
}
type CustomType struct {
	Schema    string
	Name      string
	TypeKind  string   // "enum", "domain", "composite", etc.
	Values    []string // for enums
	BaseType  string   // for domains
	Check     string   // for domain constraints
	Cols      []Column // for composite types
	Collation string   // for domains
	Default   string   // for domains
	NotNull   bool     // for domains
}

var migrationDir = pflag.StringP("migrations", "m", "", "path to migration files or directory")

func main() {
	pflag.Parse()
	if *migrationDir != "" {
		err := runLocal(*migrationDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed: %s\n", err.Error())
			os.Exit(1)
		}
		return
	}
	// If no migration path was set, it's running as a plugin.
	runPlugin()
}

func runLocal(dir string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	files := walkMigrations([]string{dir})
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

	tables, tableLevelFKs, views, customTypes, err := parseFiles(files)
	if err != nil {
		return nil, fmt.Errorf("failed to parse files: %s", err)
	}

	g, err := renderD2(tables, *tableLevelFKs, views, customTypes)
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
	themeID := int64(d2themescatalog.NeutralGrey.ID)
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
