# sqlc-viz-plugin

A sqlc compatible plugin that generates d2 graphs based on [tern](https://github.com/jackc/tern/) schemas.

## Run it stand-alone
```sh
./sqlc-viz-plugin -m "dir/with/migrations"
```

## Run it as a sqlc plugin
`sqlc.yaml`:
```
version: "2"

plugins:
  - name: viz
    process:
      cmd: ./sqlc-viz-plugin

sql:
  - schema: "path/to/migrations"
    codegen:
    - out: gen
      plugin: viz
    queries: "my/sql/queries"
    engine: "postgresql"
    gen:
      go:
      ...
```

Run as usual
```
sqlc generate
```
d2 files will be in the `out` directory, `gen`:
```
ls -p gen
schema.d2  schema.svg
```
