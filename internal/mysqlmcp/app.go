package mysqlmcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Showichiro/mysql-mcp/internal/config"
	"github.com/Showichiro/mysql-mcp/internal/sqlguard"
	"github.com/go-sql-driver/mysql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type App struct {
	cfg config.Config
	db  *sql.DB
	log *log.Logger
}

func New(cfg config.Config) (*App, error) {
	mysqlCfg := mysql.NewConfig()
	mysqlCfg.User = cfg.User
	mysqlCfg.Passwd = cfg.Password
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	mysqlCfg.DBName = cfg.Database
	mysqlCfg.ParseTime = true
	mysqlCfg.Timeout = cfg.Timeout
	mysqlCfg.ReadTimeout = cfg.Timeout
	mysqlCfg.WriteTimeout = cfg.Timeout
	if cfg.SSL {
		mysqlCfg.TLSConfig = "true"
	}

	db, err := sql.Open("mysql", mysqlCfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql connection failed")
	}

	return &App{
		cfg: cfg,
		db:  db,
		log: log.New(os.Stderr, "mysql-mcp ", log.LstdFlags),
	}, nil
}

func (a *App) Close() {
	_ = a.db.Close()
}

func (a *App) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mysql_list_schemas",
		Title:       "List MySQL schemas",
		Description: "List schemas visible to the configured read-only MySQL user.",
	}, a.listSchemas)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mysql_list_tables",
		Title:       "List MySQL tables",
		Description: "List tables in a MySQL schema.",
	}, a.listTables)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mysql_describe_table",
		Title:       "Describe MySQL table",
		Description: "Return columns and indexes for a MySQL table.",
	}, a.describeTable)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mysql_query",
		Title:       "Run read-only MySQL query",
		Description: "Run one bounded read-only SQL statement. Only SELECT, SHOW, DESCRIBE, and EXPLAIN are allowed.",
	}, a.query)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mysql_explain",
		Title:       "Explain MySQL SELECT",
		Description: "Run EXPLAIN for a SELECT statement.",
	}, a.explain)

	server.AddResource(&mcp.Resource{
		URI:         "mysql://schemas",
		Name:        "schemas",
		Title:       "MySQL schemas",
		Description: "Schemas visible to this MCP server.",
		MIMEType:    "application/json",
	}, a.readResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "mysql://schema/{schema}/tables",
		Name:        "schema_tables",
		Title:       "MySQL schema tables",
		Description: "Tables in a schema.",
		MIMEType:    "application/json",
	}, a.readResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "mysql://schema/{schema}/table/{table}",
		Name:        "table_definition",
		Title:       "MySQL table definition",
		Description: "Columns and indexes for a table.",
		MIMEType:    "application/json",
	}, a.readResource)
}

type EmptyInput struct{}

type ListSchemasOutput struct {
	Schemas []string `json:"schemas"`
}

func (a *App) listSchemas(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, ListSchemasOutput, error) {
	schemas, err := a.schemas(ctx)
	return nil, ListSchemasOutput{Schemas: schemas}, err
}

type ListTablesInput struct {
	Schema string `json:"schema" jsonschema:"MySQL schema name"`
}

type TableInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	TableRows  *int64 `json:"tableRows,omitempty"`
	CreateTime string `json:"createTime,omitempty"`
}

type ListTablesOutput struct {
	Schema string      `json:"schema"`
	Tables []TableInfo `json:"tables"`
}

func (a *App) listTables(ctx context.Context, _ *mcp.CallToolRequest, in ListTablesInput) (*mcp.CallToolResult, ListTablesOutput, error) {
	if err := a.requireSchema(in.Schema); err != nil {
		return nil, ListTablesOutput{}, err
	}
	rows, err := a.withTimeout(ctx, func(ctx context.Context) (*sql.Rows, error) {
		return a.db.QueryContext(ctx, `
			SELECT table_name, table_type, table_rows, create_time
			FROM information_schema.tables
			WHERE table_schema = ?
			ORDER BY table_name`, in.Schema)
	})
	if err != nil {
		return nil, ListTablesOutput{}, sanitizeErr("list tables failed", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var item TableInfo
		var tableRows sql.NullInt64
		var createTime sql.NullTime
		if err := rows.Scan(&item.Name, &item.Type, &tableRows, &createTime); err != nil {
			return nil, ListTablesOutput{}, sanitizeErr("scan tables failed", err)
		}
		if tableRows.Valid {
			item.TableRows = &tableRows.Int64
		}
		if createTime.Valid {
			item.CreateTime = createTime.Time.Format(time.RFC3339)
		}
		tables = append(tables, item)
	}
	return nil, ListTablesOutput{Schema: in.Schema, Tables: tables}, rows.Err()
}

type DescribeTableInput struct {
	Schema string `json:"schema" jsonschema:"MySQL schema name"`
	Table  string `json:"table" jsonschema:"MySQL table name"`
}

type ColumnInfo struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	Default      *string `json:"default,omitempty"`
	Extra        string  `json:"extra,omitempty"`
	Comment      string  `json:"comment,omitempty"`
	Ordinal      int     `json:"ordinal"`
	CharacterSet *string `json:"characterSet,omitempty"`
	Collation    *string `json:"collation,omitempty"`
}

type IndexInfo struct {
	Name        string `json:"name"`
	NonUnique   bool   `json:"nonUnique"`
	SeqInIndex  int    `json:"seqInIndex"`
	ColumnName  string `json:"columnName"`
	IndexType   string `json:"indexType"`
	Cardinality *int64 `json:"cardinality,omitempty"`
}

type DescribeTableOutput struct {
	Schema  string       `json:"schema"`
	Table   string       `json:"table"`
	Columns []ColumnInfo `json:"columns"`
	Indexes []IndexInfo  `json:"indexes"`
}

func (a *App) describeTable(ctx context.Context, _ *mcp.CallToolRequest, in DescribeTableInput) (*mcp.CallToolResult, DescribeTableOutput, error) {
	if err := a.requireSchema(in.Schema); err != nil {
		return nil, DescribeTableOutput{}, err
	}
	if in.Table == "" {
		return nil, DescribeTableOutput{}, fmt.Errorf("table is required")
	}
	columns, err := a.columns(ctx, in.Schema, in.Table)
	if err != nil {
		return nil, DescribeTableOutput{}, err
	}
	indexes, err := a.indexes(ctx, in.Schema, in.Table)
	if err != nil {
		return nil, DescribeTableOutput{}, err
	}
	return nil, DescribeTableOutput{
		Schema:  in.Schema,
		Table:   in.Table,
		Columns: columns,
		Indexes: indexes,
	}, nil
}

type QueryInput struct {
	SQL       string `json:"sql" jsonschema:"Single read-only SQL statement"`
	MaxRows   int    `json:"maxRows,omitempty" jsonschema:"Optional per-call row limit, capped by server max"`
	TimeoutMs int    `json:"timeoutMs,omitempty" jsonschema:"Optional per-call timeout in milliseconds, capped by server max"`
}

type QueryOutput struct {
	SQL             string           `json:"sql"`
	Kind            string           `json:"kind"`
	Columns         []string         `json:"columns"`
	Rows            []map[string]any `json:"rows"`
	RowCount        int              `json:"rowCount"`
	Truncated       bool             `json:"truncated"`
	ExecutionTimeMs int64            `json:"executionTimeMs"`
}

func (a *App) query(ctx context.Context, _ *mcp.CallToolRequest, in QueryInput) (*mcp.CallToolResult, QueryOutput, error) {
	maxRows := a.maxRows(in.MaxRows)
	checked, err := sqlguard.CheckReadOnly(in.SQL, maxRows)
	if err != nil {
		return nil, QueryOutput{}, err
	}
	out, err := a.runQuery(ctx, checked.SQL, string(checked.Kind), maxRows, in.TimeoutMs)
	if err != nil {
		return nil, QueryOutput{}, err
	}
	return nil, out, nil
}

type ExplainInput struct {
	SQL string `json:"sql" jsonschema:"SELECT statement to explain"`
}

type ExplainOutput struct {
	SQL     string           `json:"sql"`
	Explain []map[string]any `json:"explain"`
}

func (a *App) explain(ctx context.Context, _ *mcp.CallToolRequest, in ExplainInput) (*mcp.CallToolResult, ExplainOutput, error) {
	sqlText, err := sqlguard.CheckExplainable(in.SQL)
	if err != nil {
		return nil, ExplainOutput{}, err
	}
	out, err := a.runQuery(ctx, "EXPLAIN "+sqlText, "explain", a.cfg.MaxRows, 0)
	if err != nil {
		return nil, ExplainOutput{}, err
	}
	return nil, ExplainOutput{SQL: sqlText, Explain: out.Rows}, nil
}

func (a *App) schemas(ctx context.Context) ([]string, error) {
	rows, err := a.withTimeout(ctx, func(ctx context.Context) (*sql.Rows, error) {
		return a.db.QueryContext(ctx, `
			SELECT schema_name
			FROM information_schema.schemata
			WHERE schema_name NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys')
			ORDER BY schema_name`)
	})
	if err != nil {
		return nil, sanitizeErr("list schemas failed", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, sanitizeErr("scan schemas failed", err)
		}
		if a.cfg.SchemaAllowed(schema) {
			schemas = append(schemas, schema)
		}
	}
	return schemas, rows.Err()
}

func (a *App) columns(ctx context.Context, schema, table string) ([]ColumnInfo, error) {
	rows, err := a.withTimeout(ctx, func(ctx context.Context) (*sql.Rows, error) {
		return a.db.QueryContext(ctx, `
			SELECT column_name, column_type, is_nullable, column_default, extra,
			       column_comment, ordinal_position, character_set_name, collation_name
			FROM information_schema.columns
			WHERE table_schema = ? AND table_name = ?
			ORDER BY ordinal_position`, schema, table)
	})
	if err != nil {
		return nil, sanitizeErr("describe columns failed", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var item ColumnInfo
		var nullable string
		var def, charset, collation sql.NullString
		if err := rows.Scan(&item.Name, &item.Type, &nullable, &def, &item.Extra, &item.Comment, &item.Ordinal, &charset, &collation); err != nil {
			return nil, sanitizeErr("scan columns failed", err)
		}
		item.Nullable = nullable == "YES"
		if def.Valid {
			item.Default = &def.String
		}
		if charset.Valid {
			item.CharacterSet = &charset.String
		}
		if collation.Valid {
			item.Collation = &collation.String
		}
		columns = append(columns, item)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table not found")
	}
	return columns, rows.Err()
}

func (a *App) indexes(ctx context.Context, schema, table string) ([]IndexInfo, error) {
	rows, err := a.withTimeout(ctx, func(ctx context.Context) (*sql.Rows, error) {
		return a.db.QueryContext(ctx, `
			SELECT index_name, non_unique, seq_in_index, column_name, index_type, cardinality
			FROM information_schema.statistics
			WHERE table_schema = ? AND table_name = ?
			ORDER BY index_name, seq_in_index`, schema, table)
	})
	if err != nil {
		return nil, sanitizeErr("describe indexes failed", err)
	}
	defer rows.Close()

	var indexes []IndexInfo
	for rows.Next() {
		var item IndexInfo
		var nonUnique int
		var cardinality sql.NullInt64
		if err := rows.Scan(&item.Name, &nonUnique, &item.SeqInIndex, &item.ColumnName, &item.IndexType, &cardinality); err != nil {
			return nil, sanitizeErr("scan indexes failed", err)
		}
		item.NonUnique = nonUnique == 1
		if cardinality.Valid {
			item.Cardinality = &cardinality.Int64
		}
		indexes = append(indexes, item)
	}
	return indexes, rows.Err()
}

func (a *App) runQuery(ctx context.Context, sqlText, kind string, maxRows, timeoutMs int) (QueryOutput, error) {
	timeout := a.timeout(timeoutMs)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	rows, err := a.db.QueryContext(ctx, sqlText)
	if err != nil {
		return QueryOutput{}, sanitizeErr("query failed", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return QueryOutput{}, sanitizeErr("read columns failed", err)
	}
	values := make([]any, len(columns))
	valuePtrs := make([]any, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	var outRows []map[string]any
	truncated := false
	for rows.Next() {
		if len(outRows) >= maxRows {
			truncated = true
			break
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return QueryOutput{}, sanitizeErr("scan rows failed", err)
		}
		row := map[string]any{}
		for i, col := range columns {
			row[col] = a.cleanValue(values[i])
		}
		outRows = append(outRows, row)
	}
	if err := rows.Err(); err != nil {
		return QueryOutput{}, sanitizeErr("read rows failed", err)
	}

	a.log.Printf("kind=%s rows=%d truncated=%t elapsed_ms=%d", kind, len(outRows), truncated, time.Since(start).Milliseconds())
	return QueryOutput{
		SQL:             sqlText,
		Kind:            kind,
		Columns:         columns,
		Rows:            outRows,
		RowCount:        len(outRows),
		Truncated:       truncated,
		ExecutionTimeMs: time.Since(start).Milliseconds(),
	}, nil
}

func (a *App) cleanValue(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case []byte:
		return truncate(string(x), a.cfg.MaxCellChars)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case string:
		return truncate(x, a.cfg.MaxCellChars)
	default:
		return x
	}
}

func (a *App) readResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	var payload any
	var err error

	switch {
	case uri == "mysql://schemas":
		var schemas []string
		schemas, err = a.schemas(ctx)
		payload = ListSchemasOutput{Schemas: schemas}
	case strings.HasPrefix(uri, "mysql://schema/") && strings.HasSuffix(uri, "/tables"):
		schema, ok := parseTablesURI(uri)
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		_, payload, err = a.listTables(ctx, nil, ListTablesInput{Schema: schema})
	case strings.HasPrefix(uri, "mysql://schema/"):
		schema, table, ok := parseTableURI(uri)
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		_, payload, err = a.describeTable(ctx, nil, DescribeTableInput{Schema: schema, Table: table})
	default:
		return nil, mcp.ResourceNotFoundError(uri)
	}
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

func (a *App) requireSchema(schema string) error {
	if schema == "" {
		return fmt.Errorf("schema is required")
	}
	if !a.cfg.SchemaAllowed(schema) {
		return fmt.Errorf("schema is not allowed")
	}
	return nil
}

func (a *App) withTimeout(ctx context.Context, fn func(context.Context) (*sql.Rows, error)) (*sql.Rows, error) {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
	defer cancel()
	return fn(ctx)
}

func (a *App) maxRows(requested int) int {
	if requested <= 0 || requested > a.cfg.MaxRows {
		return a.cfg.MaxRows
	}
	return requested
}

func (a *App) timeout(requestedMs int) time.Duration {
	if requestedMs <= 0 {
		return a.cfg.Timeout
	}
	requested := time.Duration(requestedMs) * time.Millisecond
	if requested > a.cfg.Timeout {
		return a.cfg.Timeout
	}
	return requested
}

func sanitizeErr(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", message)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}

func parseTablesURI(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if u.Host != "schema" || len(parts) != 2 || parts[1] != "tables" {
		return "", false
	}
	schema, err := url.PathUnescape(parts[0])
	return schema, err == nil
}

func parseTableURI(raw string) (string, string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if u.Host != "schema" || len(parts) != 3 || parts[1] != "table" {
		return "", "", false
	}
	schema, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", "", false
	}
	table, err := url.PathUnescape(parts[2])
	if err != nil {
		return "", "", false
	}
	return schema, table, true
}
